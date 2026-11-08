// Package storage описывает интерфейсы хранилищ сервера.
//
// Реализации:
//   - storage/memory   — in-memory, данные живут до перезапуска процесса;
//   - storage/sqlstore — SQL (PostgreSQL или SQLite) с миграциями golang-migrate.
package storage

import (
	"context"
	"errors"

	"github.com/sapfirmoscow/chat-server-go/internal/models"
)

var (
	// ErrNotFound — запрошенная запись не существует.
	ErrNotFound = errors.New("not found")
	// ErrUsernameTaken — пользователь с таким username уже зарегистрирован.
	ErrUsernameTaken = errors.New("username already taken")
	// ErrInvalidChat — чат не удовлетворяет требованиям операции
	// (например, у личного чата не ровно два участника).
	ErrInvalidChat = errors.New("invalid chat")
)

// Users хранит пользователей. Username сравнивается без учёта регистра.
type Users interface {
	// Add сохраняет пользователя; ErrUsernameTaken, если username занят.
	Add(ctx context.Context, user *models.User) error
	// GetByID возвращает пользователя или ErrNotFound.
	GetByID(ctx context.Context, id string) (*models.User, error)
	// GetByUsername возвращает пользователя или ErrNotFound.
	GetByUsername(ctx context.Context, username string) (*models.User, error)
}

// Chats хранит чаты и их участников.
type Chats interface {
	// GetOrCreateDirect атомарно находит личный чат между двумя участниками
	// chat.MemberIDs или создаёт chat, если такого ещё нет.
	// created == true, если чат был создан этим вызовом.
	// ErrInvalidChat, если участников не ровно два.
	GetOrCreateDirect(ctx context.Context, chat *models.Chat) (result *models.Chat, created bool, err error)
	// GetByID возвращает чат или ErrNotFound.
	GetByID(ctx context.Context, id string) (*models.Chat, error)
	// GetUserChats возвращает чаты пользователя, новые первыми.
	GetUserChats(ctx context.Context, userID string) ([]*models.Chat, error)
}

// Messages хранит сообщения.
type Messages interface {
	// Add сохраняет сообщение.
	Add(ctx context.Context, msg *models.Message) error
	// GetByChat возвращает последние limit сообщений чата в хронологическом порядке.
	GetByChat(ctx context.Context, chatID string, limit int) ([]*models.Message, error)
}

// DirectKey — ключ уникальности личного чата: не зависит от порядка участников.
func DirectKey(userA, userB string) string {
	if userA > userB {
		userA, userB = userB, userA
	}
	return userA + ":" + userB
}
