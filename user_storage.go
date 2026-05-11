package main

import (
	"errors"
	"strings"
	"sync"
)

var ErrUsernameTaken = errors.New("username already taken")

type UserStorage struct {
	mu              sync.RWMutex
	usersByID       map[string]*User
	usersByUsername map[string]*User
}

func NewUserStorage() *UserStorage {
	return &UserStorage{
		usersByID:       make(map[string]*User),
		usersByUsername: make(map[string]*User),
	}
}

func (s *UserStorage) Add(user *User) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	//check if it already exist
	key := strings.ToLower(user.Username)

	_, exist := s.usersByUsername[key]
	if exist {
		return ErrUsernameTaken
	}

	s.usersByUsername[key] = user
	s.usersByID[user.ID] = user

	return nil
}

func (s *UserStorage) GetByID(id string) (*User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	check, ok := s.usersByID[id]

	return check, ok
}

func (s *UserStorage) GetByUsername(username string) (*User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := strings.ToLower(username)

	check, ok := s.usersByUsername[key]

	return check, ok
}
