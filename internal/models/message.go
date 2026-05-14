package models

import "time"

type Message struct {
	ID        string
	ChatID    string
	SenderID  string
	Text      string
	CreatedAt time.Time
}

type PublicMessage struct {
	ID        string    `json:"id"`
	ChatID    string    `json:"chat_id"`
	SenderID  string    `json:"sender_id"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"created_at"`
}

func (m *Message) ToPublic() PublicMessage {
	return PublicMessage{
		ID:        m.ID,
		ChatID:    m.ChatID,
		SenderID:  m.SenderID,
		Text:      m.Text,
		CreatedAt: m.CreatedAt,
	}
}
