package storage

import (
	"errors"
	"sync"

	"github.com/sapfirmoscow/chat-server-go/internal/models"
)

var ErrChatAlreadyExists = errors.New("chat already exists")

type Chats struct {
	mu        sync.RWMutex
	chatsByID map[string]*models.Chat
}

func NewChats() *Chats {
	return &Chats{
		chatsByID: make(map[string]*models.Chat),
	}
}

func (c *Chats) Add(chat *models.Chat) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	_, exist := c.chatsByID[chat.ID]

	if exist {
		return ErrChatAlreadyExists
	}

	c.chatsByID[chat.ID] = chat

	return nil
}

func (c *Chats) GetByID(id string) (*models.Chat, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	chat, ok := c.chatsByID[id]
	return chat, ok
}

func (c *Chats) GetUserChats(userID string) []*models.Chat {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]*models.Chat, 0)

	for _, chat := range c.chatsByID {
		if chat.HasMember(userID) {
			result = append(result, chat)
		}
	}
	return result
}

func (c *Chats) FindDirectChat(userA, userB string) (*models.Chat, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, chat := range c.chatsByID {
		if len(chat.MemberIDs) == 2 && //2 - chat for both(private)
			chat.HasMember(userA) && chat.HasMember(userB) {
			return chat, true
		}
	}
	return nil, false
}
