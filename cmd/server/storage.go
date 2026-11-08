package main

import (
	"context"
	"fmt"
	"os"

	"github.com/sapfirmoscow/chat-server-go/internal/storage"
	"github.com/sapfirmoscow/chat-server-go/internal/storage/memory"
	"github.com/sapfirmoscow/chat-server-go/internal/storage/sqlstore"
)

const defaultSQLitePath = "chat.db"

// stores — выбранная реализация хранилищ и функция освобождения ресурсов.
type stores struct {
	users    storage.Users
	chats    storage.Chats
	messages storage.Messages
	close    func() error
}

// openStorage выбирает хранилище по переменным окружения:
//
//	STORAGE_DRIVER — memory | sqlite | postgres (по умолчанию memory)
//	DATABASE_URL   — для sqlite путь к файлу (по умолчанию chat.db),
//	                 для postgres обязательный URL подключения
func openStorage(ctx context.Context) (*stores, string, error) {
	driver := os.Getenv("STORAGE_DRIVER")
	if driver == "" {
		driver = "memory"
	}
	dsn := os.Getenv("DATABASE_URL")

	switch driver {
	case "memory":
		return &stores{
			users:    memory.NewUsers(),
			chats:    memory.NewChats(),
			messages: memory.NewMessages(),
			close:    func() error { return nil },
		}, "memory", nil

	case string(sqlstore.SQLite), string(sqlstore.Postgres):
		dialect := sqlstore.Dialect(driver)
		if dsn == "" {
			if dialect == sqlstore.Postgres {
				return nil, "", fmt.Errorf("DATABASE_URL is required for STORAGE_DRIVER=postgres")
			}
			dsn = defaultSQLitePath
		}

		s, err := sqlstore.Open(ctx, dialect, dsn)
		if err != nil {
			return nil, "", err
		}

		desc := driver
		if dialect == sqlstore.SQLite {
			desc += " (" + dsn + ")" // путь к файлу не секрет, в отличие от URL с паролем
		}
		return &stores{
			users:    s.Users,
			chats:    s.Chats,
			messages: s.Messages,
			close:    s.Close,
		}, desc, nil

	default:
		return nil, "", fmt.Errorf("unknown STORAGE_DRIVER %q (want memory, sqlite or postgres)", driver)
	}
}
