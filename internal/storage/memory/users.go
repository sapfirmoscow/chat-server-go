// Package memory — in-memory реализация интерфейсов storage.
// Данные живут только до перезапуска процесса.
package memory

import (
	"context"
	"strings"
	"sync"

	"github.com/sapfirmoscow/chat-server-go/internal/models"
	"github.com/sapfirmoscow/chat-server-go/internal/storage"
)

var _ storage.Users = (*Users)(nil)

type Users struct {
	mu              sync.RWMutex
	usersByID       map[string]*models.User
	usersByUsername map[string]*models.User
}

func NewUsers() *Users {
	return &Users{
		usersByID:       make(map[string]*models.User),
		usersByUsername: make(map[string]*models.User),
	}
}

func (u *Users) Add(_ context.Context, user *models.User) error {
	u.mu.Lock()
	defer u.mu.Unlock()

	key := strings.ToLower(user.Username)

	if _, exist := u.usersByUsername[key]; exist {
		return storage.ErrUsernameTaken
	}

	u.usersByUsername[key] = user
	u.usersByID[user.ID] = user

	return nil
}

func (u *Users) GetByID(_ context.Context, id string) (*models.User, error) {
	u.mu.RLock()
	defer u.mu.RUnlock()

	user, ok := u.usersByID[id]
	if !ok {
		return nil, storage.ErrNotFound
	}
	return user, nil
}

func (u *Users) GetByUsername(_ context.Context, username string) (*models.User, error) {
	u.mu.RLock()
	defer u.mu.RUnlock()

	user, ok := u.usersByUsername[strings.ToLower(username)]
	if !ok {
		return nil, storage.ErrNotFound
	}
	return user, nil
}
