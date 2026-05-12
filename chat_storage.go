package main

import (
	"errors"
	"sync"
)

var ErrChatAlreadyExists = errors.New("chat already exists")

type ChatStorage struct {
	mu        sync.RWMutex
	chatsByID map[string]*Chat
}

func NewChatStorage() *ChatStorage {
	return &ChatStorage{
		chatsByID: make(map[string]*Chat),
	}
}

func (cs *ChatStorage) Add(chat *Chat) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	_, exist := cs.chatsByID[chat.ID]

	if exist {
		return ErrChatAlreadyExists
	}

	cs.chatsByID[chat.ID] = chat

	return nil
}

func (cs *ChatStorage) GetByID(id string) (*Chat, bool) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	chat, ok := cs.chatsByID[id]
	return chat, ok
}

func (cs *ChatStorage) GetUserChats(userID string) []*Chat {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	result := make([]*Chat, 0)

	for _, chat := range cs.chatsByID {
		if chat.HasMember(userID) {
			result = append(result, chat)
		}
	}
	return result
}

func (cs *ChatStorage) FindDirectChat(userA, userB string) (*Chat, bool) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	for _, chat := range cs.chatsByID {
		if len(chat.MemberIDs) == 2 && //2 - chat for both(private)
			chat.HasMember(userA) && chat.HasMember(userB) {
			return chat, true
		}
	}
	return nil, false
}
