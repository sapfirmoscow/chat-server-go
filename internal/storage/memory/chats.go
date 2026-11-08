package memory

import (
	"context"
	"sort"
	"sync"

	"github.com/sapfirmoscow/chat-server-go/internal/models"
	"github.com/sapfirmoscow/chat-server-go/internal/storage"
)

var _ storage.Chats = (*Chats)(nil)

type Chats struct {
	mu          sync.RWMutex
	chatsByID   map[string]*models.Chat
	directByKey map[string]*models.Chat
}

func NewChats() *Chats {
	return &Chats{
		chatsByID:   make(map[string]*models.Chat),
		directByKey: make(map[string]*models.Chat),
	}
}

// GetOrCreateDirect ищет и создаёт под одной блокировкой, поэтому два
// одновременных запроса не создадут два чата для одной пары.
func (c *Chats) GetOrCreateDirect(_ context.Context, chat *models.Chat) (*models.Chat, bool, error) {
	if len(chat.MemberIDs) != 2 || chat.MemberIDs[0] == chat.MemberIDs[1] {
		return nil, false, storage.ErrInvalidChat
	}
	key := storage.DirectKey(chat.MemberIDs[0], chat.MemberIDs[1])

	c.mu.Lock()
	defer c.mu.Unlock()

	if existing, ok := c.directByKey[key]; ok {
		return existing, false, nil
	}

	c.chatsByID[chat.ID] = chat
	c.directByKey[key] = chat
	return chat, true, nil
}

func (c *Chats) GetByID(_ context.Context, id string) (*models.Chat, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	chat, ok := c.chatsByID[id]
	if !ok {
		return nil, storage.ErrNotFound
	}
	return chat, nil
}

func (c *Chats) GetUserChats(_ context.Context, userID string) ([]*models.Chat, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]*models.Chat, 0)
	for _, chat := range c.chatsByID {
		if chat.HasMember(userID) {
			result = append(result, chat)
		}
	}

	// Новые первыми; при равном времени — стабильно по ID.
	sort.Slice(result, func(i, j int) bool {
		if !result[i].CreatedAt.Equal(result[j].CreatedAt) {
			return result[i].CreatedAt.After(result[j].CreatedAt)
		}
		return result[i].ID < result[j].ID
	})
	return result, nil
}
