package main

import "sync"

type MessageStorage struct {
	mu       sync.RWMutex
	messages []*Message
}

func NewMessageStorage() *MessageStorage {
	return &MessageStorage{
		messages: make([]*Message, 0),
	}
}

func (ms *MessageStorage) Add(msg *Message) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	ms.messages = append(ms.messages, msg)
}

func (ms *MessageStorage) GetByChat(chatID string, limit int) []*Message {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	chatMessages := make([]*Message, 0)
	//linear
	for _, msg := range ms.messages {
		if msg.ChatID == chatID {
			chatMessages = append(chatMessages, msg)
		}
	}

	if len(chatMessages) > limit {
		chatMessages = chatMessages[len(chatMessages)-limit:]
	}
	return chatMessages
}
