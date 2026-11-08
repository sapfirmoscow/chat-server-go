package sqlstore

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database"
	migratepgx "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	migratesqlite "github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed migrations/postgres/*.sql migrations/sqlite/*.sql
var migrationsFS embed.FS

// migrateUp применяет все непримененные миграции.
//
// Миграции идут через отдельное соединение: драйверы golang-migrate держат
// своё соединение и закрывают *sql.DB в Close, поэтому основной пул им не отдаётся.
func migrateUp(dialect Dialect, driver, dsn string) (err error) {
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return fmt.Errorf("sqlstore: open for migrations: %w", err)
	}

	var dbDriver database.Driver
	switch dialect {
	case Postgres:
		dbDriver, err = migratepgx.WithInstance(db, &migratepgx.Config{})
	case SQLite:
		dbDriver, err = migratesqlite.WithInstance(db, &migratesqlite.Config{})
	default:
		err = fmt.Errorf("sqlstore: unsupported dialect %q", dialect)
	}
	if err != nil {
		_ = db.Close()
		return fmt.Errorf("sqlstore: migration driver: %w", err)
	}

	src, err := iofs.New(migrationsFS, "migrations/"+string(dialect))
	if err != nil {
		_ = dbDriver.Close()
		return fmt.Errorf("sqlstore: migration source: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", src, string(dialect), dbDriver)
	if err != nil {
		_ = src.Close()
		_ = dbDriver.Close()
		return fmt.Errorf("sqlstore: migrate init: %w", err)
	}
	// m.Close закрывает и источник, и драйвер БД (а вместе с ним db).
	defer func() {
		srcErr, dbErr := m.Close()
		if err == nil {
			err = errors.Join(srcErr, dbErr)
		}
	}()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("sqlstore: migrate up: %w", err)
	}
	return nil
}
