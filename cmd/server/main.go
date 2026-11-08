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
	"github.com/sapfirmoscow/chat-server-go/internal/handlers"
	"github.com/sapfirmoscow/chat-server-go/internal/ratelimit"
	"github.com/sapfirmoscow/chat-server-go/internal/ws"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	store, storageDesc, err := openStorage(ctx)
	if err != nil {
		log.Fatalf("storage: %v", err)
	}
	defer func() {
		if err := store.close(); err != nil {
			log.Printf("storage close: %v", err)
		}
	}()
	log.Printf("storage: %s", storageDesc)

	userStorage, chatStorage, messages := store.users, store.chats, store.messages

	jwtManager := auth.NewManager("i will change it for prom", time.Hour*24)

	authMW := auth.Middleware(jwtManager)

	hub := ws.NewHub()

	// Поиск пользователей ограничен по частоте: гасит автоматический перебор username.
	userSearchLimiter := ratelimit.New(20, time.Minute)

	http.HandleFunc("POST /register", handlers.HandleRegister(userStorage, jwtManager))
	http.HandleFunc("POST /login", handlers.HandleLogin(userStorage, jwtManager))
	http.Handle("GET /me", authMW(http.HandlerFunc(handlers.HandleMe(userStorage))))

	http.Handle("GET /users/by-username/{username}",
		authMW(userSearchLimiter.Middleware(
			http.HandlerFunc(handlers.HandleFindUserByUsername(userStorage)))))

	http.Handle("POST /chats",
		authMW(http.HandlerFunc(handlers.HandleCreateChat(chatStorage, userStorage))))

	http.Handle("GET /chats",
		authMW(http.HandlerFunc(handlers.HandleGetMyChats(chatStorage, userStorage))))

	http.Handle("POST /chats/{id}/messages",
		authMW(http.HandlerFunc(handlers.HandleSendMessage(chatStorage, messages, hub))))

	http.Handle("GET /chats/{id}/messages",
		authMW(http.HandlerFunc(handlers.HandleGetMessages(chatStorage, messages))))

	http.Handle("GET /ws", handlers.HandleWS(jwtManager, hub))

	srv := &http.Server{Addr: ":8080"}

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

	log.Println("Server started on http://localhost:8080")
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Printf("server: %v", err)
		return // defer закроет хранилище
	}
	log.Println("Server stopped")
}
