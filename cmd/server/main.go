package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sapfirmoscow/chat-server-go/internal/auth"
	"github.com/sapfirmoscow/chat-server-go/internal/ws"
)

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

	go func() {
		<-ctx.Done()
		// Перестаём принимать соединения и ждём текущие HTTP-запросы.
		// WebSocket-соединения Shutdown не ждёт: они закроются с процессом.
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("shutdown: %v", err)
		}
	}()

	log.Printf("Server started on %s", cfg.Addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Printf("server: %v", err)
		return // defer закроет хранилище
	}
	log.Println("Server stopped")
}
