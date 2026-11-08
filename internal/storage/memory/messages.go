package memory

import (
	"context"
	"sync"

	"github.com/sapfirmoscow/chat-server-go/internal/models"
	"github.com/sapfirmoscow/chat-server-go/internal/storage"
)

var _ storage.Messages = (*Messages)(nil)

type Messages struct {
	mu     sync.RWMutex
	byChat map[string][]*models.Message
}

func NewMessages() *Messages {
	return &Messages{
		byChat: make(map[string][]*models.Message),
	}
}

func (m *Messages) Add(_ context.Context, msg *models.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.byChat[msg.ChatID] = append(m.byChat[msg.ChatID], msg)
	return nil
}

func (m *Messages) GetByChat(_ context.Context, chatID string, limit int) ([]*models.Message, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	all := m.byChat[chatID]
	if limit > 0 && len(all) > limit {
		all = all[len(all)-limit:]
	}

	// Копия среза: вызывающий не должен видеть последующие append.
	result := make([]*models.Message, len(all))
	copy(result, all)
	return result, nil
}
