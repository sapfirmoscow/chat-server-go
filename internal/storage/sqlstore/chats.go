package sqlstore

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/sapfirmoscow/chat-server-go/internal/models"
	"github.com/sapfirmoscow/chat-server-go/internal/storage"
)

var _ storage.Chats = (*Chats)(nil)

type Chats struct {
	*base
}

// GetOrCreateDirect вставляет чат с уникальным direct_key. Если такой ключ уже
// есть, INSERT ничего не делает, и возвращается существующий чат.
//
// В PostgreSQL параллельный INSERT с тем же ключом ждёт коммита первой
// транзакции, после чего получает конфликт, поэтому дубликатов не бывает.
// В SQLite запись и так сериализована.
func (c *Chats) GetOrCreateDirect(ctx context.Context, chat *models.Chat) (*models.Chat, bool, error) {
	if len(chat.MemberIDs) != 2 || chat.MemberIDs[0] == chat.MemberIDs[1] {
		return nil, false, storage.ErrInvalidChat
	}
	key := storage.DirectKey(chat.MemberIDs[0], chat.MemberIDs[1])

	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, false, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }() // после Commit — no-op

	res, err := tx.ExecContext(ctx, c.q(`
		INSERT INTO chats (id, direct_key, created_at)
		VALUES (?, ?, ?)
		ON CONFLICT (direct_key) DO NOTHING`),
		chat.ID, key, chat.CreatedAt.UTC(),
	)
	if err != nil {
		return nil, false, fmt.Errorf("insert chat: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return nil, false, fmt.Errorf("insert chat: rows affected: %w", err)
	}

	if n == 0 {
		// Чат уже существует. Транзакцию закрываем до чтения: у SQLite одно
		// соединение в пуле, и запрос мимо tx иначе ждал бы его вечно.
		_ = tx.Rollback()

		var id string
		err := c.db.QueryRowContext(ctx, c.q(`SELECT id FROM chats WHERE direct_key = ?`), key).Scan(&id)
		if err != nil {
			return nil, false, fmt.Errorf("select existing chat: %w", err)
		}
		existing, err := c.GetByID(ctx, id)
		if err != nil {
			return nil, false, err
		}
		return existing, false, nil
	}

	for i, userID := range chat.MemberIDs {
		_, err := tx.ExecContext(ctx, c.q(`
			INSERT INTO chat_members (chat_id, user_id, position)
			VALUES (?, ?, ?)`),
			chat.ID, userID, i,
		)
		if err != nil {
			return nil, false, fmt.Errorf("insert chat member: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, false, fmt.Errorf("commit: %w", err)
	}
	return chat, true, nil
}

func (c *Chats) GetByID(ctx context.Context, id string) (*models.Chat, error) {
	rows, err := c.db.QueryContext(ctx, c.q(`
		SELECT c.id, c.created_at, m.user_id
		FROM chats c
		JOIN chat_members m ON m.chat_id = c.id
		WHERE c.id = ?
		ORDER BY m.position`), id)
	if err != nil {
		return nil, fmt.Errorf("select chat: %w", err)
	}

	chats, err := scanChats(rows)
	if err != nil {
		return nil, err
	}
	if len(chats) == 0 {
		return nil, storage.ErrNotFound
	}
	return chats[0], nil
}

func (c *Chats) GetUserChats(ctx context.Context, userID string) ([]*models.Chat, error) {
	rows, err := c.db.QueryContext(ctx, c.q(`
		SELECT c.id, c.created_at, m.user_id
		FROM chat_members me
		JOIN chats c        ON c.id = me.chat_id
		JOIN chat_members m ON m.chat_id = c.id
		WHERE me.user_id = ?
		ORDER BY c.created_at DESC, c.id, m.position`), userID)
	if err != nil {
		return nil, fmt.Errorf("select user chats: %w", err)
	}
	return scanChats(rows)
}

// scanChats собирает строки (chat_id, created_at, member_id), отсортированные
// по чату, в список чатов с участниками. Закрывает rows.
func scanChats(rows *sql.Rows) ([]*models.Chat, error) {
	defer rows.Close()

	result := make([]*models.Chat, 0)
	var current *models.Chat

	for rows.Next() {
		var (
			id, memberID string
			createdAt    time.Time
		)
		if err := rows.Scan(&id, &createdAt, &memberID); err != nil {
			return nil, fmt.Errorf("scan chat: %w", err)
		}
		if current == nil || current.ID != id {
			current = &models.Chat{ID: id, CreatedAt: createdAt.UTC()}
			result = append(result, current)
		}
		current.MemberIDs = append(current.MemberIDs, memberID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate chats: %w", err)
	}
	return result, nil
}
