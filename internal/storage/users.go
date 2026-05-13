package storage

import (
	"errors"
	"strings"
	"sync"

	"github.com/sapfirmoscow/chat-server-go/internal/models"
)

var ErrUsernameTaken = errors.New("username already taken")

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

func (u *Users) Add(user *models.User) error {
	u.mu.Lock()
	defer u.mu.Unlock()

	//check if it already exist
	key := strings.ToLower(user.Username)

	_, exist := u.usersByUsername[key]
	if exist {
		return ErrUsernameTaken
	}

	u.usersByUsername[key] = user
	u.usersByID[user.ID] = user

	return nil
}

func (u *Users) GetByID(id string) (*models.User, bool) {
	u.mu.RLock()
	defer u.mu.RUnlock()

	check, ok := u.usersByID[id]

	return check, ok
}

func (u *Users) GetByUsername(username string) (*models.User, bool) {
	u.mu.RLock()
	defer u.mu.RUnlock()

	key := strings.ToLower(username)

	check, ok := u.usersByUsername[key]

	return check, ok
}
