// Package sqlstore — реализация интерфейсов storage поверх database/sql.
//
// Поддерживаются два диалекта:
//   - PostgreSQL (драйвер jackc/pgx/v5);
//   - SQLite (драйвер modernc.org/sqlite, чистый Go, без CGO).
//
// Запросы пишутся с плейсхолдерами "?" и для PostgreSQL переписываются в "$1, $2, ...".
// Схема создаётся миграциями golang-migrate (см. migrate.go и каталог migrations/).
package sqlstore

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib" // драйвер "pgx"
	_ "modernc.org/sqlite"             // драйвер "sqlite"
)

// Dialect — поддерживаемая СУБД.
type Dialect string

const (
	Postgres Dialect = "postgres"
	SQLite   Dialect = "sqlite"
)

func (d Dialect) driverName() (string, error) {
	switch d {
	case Postgres:
		return "pgx", nil
	case SQLite:
		return "sqlite", nil
	default:
		return "", fmt.Errorf("sqlstore: unsupported dialect %q", d)
	}
}

// Store — подключение к БД и хранилища поверх него.
type Store struct {
	db *sql.DB

	Users    *Users
	Chats    *Chats
	Messages *Messages
}

// Open применяет миграции и открывает пул соединений.
//
// dsn для PostgreSQL — URL вида postgres://user:pass@host:5432/db?sslmode=disable,
// для SQLite — путь к файлу (например, chat.db). Для SQLite к dsn автоматически
// добавляются pragma (foreign_keys, busy_timeout, journal_mode=WAL) и _time_format=sqlite.
func Open(ctx context.Context, dialect Dialect, dsn string) (*Store, error) {
	driver, err := dialect.driverName()
	if err != nil {
		return nil, err
	}
	if dsn == "" {
		return nil, fmt.Errorf("sqlstore: empty dsn for %s", dialect)
	}
	if dialect == SQLite {
		dsn = sqliteDSN(dsn)
	}

	if err := migrateUp(dialect, driver, dsn); err != nil {
		return nil, err
	}

	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("sqlstore: open: %w", err)
	}

	if dialect == SQLite {
		// SQLite допускает одного писателя. Одно соединение сериализует запросы
		// внутри процесса и исключает SQLITE_BUSY между своими же соединениями.
		db.SetMaxOpenConns(1)
	}

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("sqlstore: ping: %w", err)
	}

	b := &base{db: db, dialect: dialect}
	return &Store{
		db:       db,
		Users:    &Users{base: b},
		Chats:    &Chats{base: b},
		Messages: &Messages{base: b},
	}, nil
}

// Close закрывает пул соединений.
func (s *Store) Close() error {
	return s.db.Close()
}

// DB даёт доступ к пулу (для тестов и диагностики).
func (s *Store) DB() *sql.DB {
	return s.db
}

func sqliteDSN(dsn string) string {
	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	// _time_format=sqlite: время хранится как "2006-01-02 15:04:05.999999999-07:00",
	// формат, понятный встроенным функциям дат SQLite.
	return dsn + sep + "_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_time_format=sqlite"
}

type base struct {
	db      *sql.DB
	dialect Dialect
}

// q приводит плейсхолдеры к синтаксису диалекта.
func (b *base) q(query string) string {
	if b.dialect != Postgres {
		return query
	}
	return rebindDollar(query)
}

// rebindDollar заменяет каждый "?" на "$N". Запросы пакета не содержат "?"
// в строковых литералах, поэтому простой замены достаточно.
func rebindDollar(query string) string {
	var sb strings.Builder
	sb.Grow(len(query) + 8)
	n := 0
	for i := 0; i < len(query); i++ {
		if query[i] == '?' {
			n++
			sb.WriteByte('$')
			sb.WriteString(strconv.Itoa(n))
			continue
		}
		sb.WriteByte(query[i])
	}
	return sb.String()
}
