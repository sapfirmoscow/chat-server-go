package sqlstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/sapfirmoscow/chat-server-go/internal/models"
	"github.com/sapfirmoscow/chat-server-go/internal/storage"
)

var _ storage.Users = (*Users)(nil)

type Users struct {
	*base
}

// Add вставляет пользователя. Конфликт по username не считается ошибкой БД:
// ON CONFLICT DO NOTHING + проверка числа вставленных строк одинаково
// работают в PostgreSQL и SQLite без разбора кодов ошибок драйверов.
func (u *Users) Add(ctx context.Context, user *models.User) error {
	res, err := u.db.ExecContext(ctx, u.q(`
		INSERT INTO users (id, username, password_hash, created_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT (username) DO NOTHING`),
		user.ID, strings.ToLower(user.Username), user.PasswordHash, user.CreatedAt.UTC(),
	)
	if err != nil {
		return fmt.Errorf("insert user: %w", err)
	}

	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("insert user: rows affected: %w", err)
	}
	if n == 0 {
		return storage.ErrUsernameTaken
	}
	return nil
}

func (u *Users) GetByID(ctx context.Context, id string) (*models.User, error) {
	return u.getOne(ctx, `
		SELECT id, username, password_hash, created_at
		FROM users WHERE id = ?`, id)
}

func (u *Users) GetByUsername(ctx context.Context, username string) (*models.User, error) {
	return u.getOne(ctx, `
		SELECT id, username, password_hash, created_at
		FROM users WHERE username = ?`, strings.ToLower(username))
}

func (u *Users) getOne(ctx context.Context, query string, arg any) (*models.User, error) {
	var user models.User
	err := u.db.QueryRowContext(ctx, u.q(query), arg).
		Scan(&user.ID, &user.Username, &user.PasswordHash, &user.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, storage.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("select user: %w", err)
	}
	user.CreatedAt = user.CreatedAt.UTC()
	return &user, nil
}
