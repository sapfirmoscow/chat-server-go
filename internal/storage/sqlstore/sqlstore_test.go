package sqlstore_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/sapfirmoscow/chat-server-go/internal/storage/sqlstore"
	"github.com/sapfirmoscow/chat-server-go/internal/storage/storagetest"
)

func stores(s *sqlstore.Store) storagetest.Stores {
	return storagetest.Stores{Users: s.Users, Chats: s.Chats, Messages: s.Messages}
}

func TestSQLiteStorage(t *testing.T) {
	storagetest.Run(t, func(t *testing.T) storagetest.Stores {
		dsn := filepath.Join(t.TempDir(), "test.db")
		s, err := sqlstore.Open(context.Background(), sqlstore.SQLite, dsn)
		if err != nil {
			t.Fatalf("open sqlite: %v", err)
		}
		// Регистрируется после TempDir, поэтому выполнится раньше удаления
		// каталога (на Windows открытый файл удалить нельзя).
		t.Cleanup(func() { _ = s.Close() })
		return stores(s)
	})
}

// Повторный Open того же файла не должен падать на уже применённых миграциях.
func TestSQLiteReopen(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "test.db")
	for i := range 2 {
		s, err := sqlstore.Open(context.Background(), sqlstore.SQLite, dsn)
		if err != nil {
			t.Fatalf("open #%d: %v", i+1, err)
		}
		_ = s.Close()
	}
}

// TestPostgresStorage запускается, только если задан TEST_POSTGRES_URL, например:
//
//	TEST_POSTGRES_URL=postgres://chat:chat@localhost:5432/chat?sslmode=disable go test ./...
//
// Внимание: тест очищает таблицы в этой БД.
func TestPostgresStorage(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_URL")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_URL not set")
	}

	storagetest.Run(t, func(t *testing.T) storagetest.Stores {
		ctx := context.Background()
		s, err := sqlstore.Open(ctx, sqlstore.Postgres, dsn)
		if err != nil {
			t.Fatalf("open postgres: %v", err)
		}
		t.Cleanup(func() { _ = s.Close() })

		if _, err := s.DB().ExecContext(ctx, `TRUNCATE users, chats, chat_members, messages RESTART IDENTITY CASCADE`); err != nil {
			t.Fatalf("truncate: %v", err)
		}
		return stores(s)
	})
}
