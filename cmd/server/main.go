package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sapfirmoscow/chat-server-go/internal/auth"
	"github.com/sapfirmoscow/chat-server-go/internal/ws"
)

// shutdownTimeout — сколько ждём текущие запросы и закрытие WebSocket при остановке.
const shutdownTimeout = 10 * time.Second

func main() {
	cfg, err := loadConfig(os.Getenv)
	if err != nil {
		log.Fatalf("config:\n%v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	store, storageDesc, err := openStorage(ctx, cfg.StorageDriver, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("storage: %v", err)
	}
	defer func() {
		if err := store.close(); err != nil {
			log.Printf("storage close: %v", err)
		}
	}()
	log.Printf("storage: %s", storageDesc)

	jwtManager := auth.NewManager(cfg.JWTSecret, time.Hour*24)
	hub := ws.NewHub()

	router := newRouter(deps{
		users:    store.users,
		chats:    store.chats,
		messages: store.messages,
		jwt:      jwtManager,
		hub:      hub,
	})
	srv := newHTTPServer(cfg.Addr, router, defaultTimeouts)

	serveErr := make(chan error, 1)
	go func() {
		log.Printf("Server started on %s", cfg.Addr)
		serveErr <- srv.ListenAndServe()
	}()

	select {
	case err := <-serveErr:
		// Сервер не поднялся (например, порт занят) — дальше ждать нечего.
		log.Printf("server: %v", err)
		return // defer закроет хранилище
	case <-ctx.Done():
	}

	log.Println("Shutting down...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	// Сначала перестаём принимать соединения и дожидаемся текущих HTTP-запросов,
	// затем закрываем WebSocket-соединения (Shutdown их не отслеживает).
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("http shutdown: %v", err)
	}
	hub.Shutdown(shutdownCtx)

	log.Println("Server stopped")
}
