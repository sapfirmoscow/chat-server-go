package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/sapfirmoscow/chat-server-go/internal/auth"
	"github.com/sapfirmoscow/chat-server-go/internal/handlers"
	"github.com/sapfirmoscow/chat-server-go/internal/storage"
)

func main() {
	userStorage := storage.NewUsers()
	chatStorage := storage.NewChats()
	messages := storage.NewMessages()

	jwtManager := auth.NewManager("i will change it for prom", time.Hour*24)

	authMW := auth.Middleware(jwtManager)

	http.HandleFunc("POST /register", handlers.HandleRegister(userStorage, jwtManager))
	http.HandleFunc("POST /login", handlers.HandleLogin(userStorage, jwtManager))
	http.Handle("GET /me", authMW(http.HandlerFunc(handlers.HandleMe(userStorage))))

	http.Handle("POST /chats",
		authMW(http.HandlerFunc(handlers.HandleCreateChat(chatStorage, userStorage))))

	http.Handle("GET /chats",
		authMW(http.HandlerFunc(handlers.HandleGetMyChats(chatStorage, userStorage))))

	http.Handle("POST /chats/{id}/messages",
		authMW(http.HandlerFunc(handlers.HandleSendMessage(chatStorage, messages))))

	http.Handle("GET /chats/{id}/messages",
		authMW(http.HandlerFunc(handlers.HandleGetMessages(chatStorage, messages))))

	fmt.Println("Server started on http://localhost:8080")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal(err)
	}
}
