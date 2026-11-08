package sqlstore

import (
	"context"
	"fmt"
	"math"
	"slices"

	"github.com/sapfirmoscow/chat-server-go/internal/models"
	"github.com/sapfirmoscow/chat-server-go/internal/storage"
)

var _ storage.Messages = (*Messages)(nil)

type Messages struct {
	*base
}

func (m *Messages) Add(ctx context.Context, msg *models.Message) error {
	_, err := m.db.ExecContext(ctx, m.q(`
		INSERT INTO messages (id, chat_id, sender_id, text, created_at)
		VALUES (?, ?, ?, ?, ?)`),
		msg.ID, msg.ChatID, msg.SenderID, msg.Text, msg.CreatedAt.UTC(),
	)
	if err != nil {
		return fmt.Errorf("insert message: %w", err)
	}
	return nil
}

// GetByChat берёт последние limit сообщений по индексу (chat_id, seq)
// и разворачивает их в хронологический порядок.
func (m *Messages) GetByChat(ctx context.Context, chatID string, limit int) ([]*models.Message, error) {
	if limit <= 0 {
		limit = math.MaxInt32 // без ограничения, как в memory
	}
	rows, err := m.db.QueryContext(ctx, m.q(`
		SELECT id, chat_id, sender_id, text, created_at
		FROM messages
		WHERE chat_id = ?
		ORDER BY seq DESC
		LIMIT ?`), chatID, limit)
	if err != nil {
		return nil, fmt.Errorf("select messages: %w", err)
	}
	defer rows.Close()

	result := make([]*models.Message, 0)
	for rows.Next() {
		var msg models.Message
		if err := rows.Scan(&msg.ID, &msg.ChatID, &msg.SenderID, &msg.Text, &msg.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		msg.CreatedAt = msg.CreatedAt.UTC()
		result = append(result, &msg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate messages: %w", err)
	}

	slices.Reverse(result)
	return result, nil
}
