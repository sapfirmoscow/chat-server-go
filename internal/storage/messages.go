package storage

import (
	"sync"

	"github.com/sapfirmoscow/chat-server-go/internal/models"
)

type Messages struct {
	mu       sync.RWMutex
	messages []*models.Message
}

func NewMessages() *Messages {
	return &Messages{
		messages: make([]*models.Message, 0),
	}
}

func (m *Messages) Add(msg *models.Message) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.messages = append(m.messages, msg)
}

func (m *Messages) GetByChat(chatID string, limit int) []*models.Message {
	m.mu.RLock()
	defer m.mu.RUnlock()

	chatMessages := make([]*models.Message, 0)
	//linear
	for _, msg := range m.messages {
		if msg.ChatID == chatID {
			chatMessages = append(chatMessages, msg)
		}
	}

	if len(chatMessages) > limit {
		chatMessages = chatMessages[len(chatMessages)-limit:]
	}
	return chatMessages
}
