package memory_test

import (
	"testing"

	"github.com/sapfirmoscow/chat-server-go/internal/storage/memory"
	"github.com/sapfirmoscow/chat-server-go/internal/storage/storagetest"
)

func TestMemoryStorage(t *testing.T) {
	storagetest.Run(t, func(t *testing.T) storagetest.Stores {
		return storagetest.Stores{
			Users:    memory.NewUsers(),
			Chats:    memory.NewChats(),
			Messages: memory.NewMessages(),
		}
	})
}
