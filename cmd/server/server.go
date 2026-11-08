package main

import (
	"net/http"
	"time"

	"github.com/sapfirmoscow/chat-server-go/internal/auth"
	"github.com/sapfirmoscow/chat-server-go/internal/handlers"
	"github.com/sapfirmoscow/chat-server-go/internal/ratelimit"
	"github.com/sapfirmoscow/chat-server-go/internal/storage"
	"github.com/sapfirmoscow/chat-server-go/internal/ws"
)

// maxBodyBytes — предел тела запроса. Самый большой легальный запрос —
// сообщение на 2000 байт текста, так что 64 КиБ хватает с большим запасом.
const maxBodyBytes = 64 << 10

// serverTimeouts — таймауты http.Server. Защищают от клиентов, которые
// держат соединение, медленно отправляя заголовки или тело (slowloris).
type serverTimeouts struct {
	ReadHeader time.Duration // на чтение заголовков запроса
	Read       time.Duration // на чтение всего запроса, включая тело
	Write      time.Duration // на запись ответа
	Idle       time.Duration // простой keep-alive соединения между запросами
}

var defaultTimeouts = serverTimeouts{
	ReadHeader: 5 * time.Second,
	Read:       15 * time.Second,
	Write:      15 * time.Second,
	Idle:       60 * time.Second,
}

// deps — зависимости обработчиков.
type deps struct {
	users    storage.Users
	chats    storage.Chats
	messages storage.Messages
	jwt      *auth.Manager
	hub      *ws.Hub
}

// newRouter регистрирует маршруты и ограничивает размер тела всех запросов.
func newRouter(d deps) http.Handler {
	mux := http.NewServeMux()
	authMW := auth.Middleware(d.jwt)

	// Поиск пользователей ограничен по частоте: гасит автоматический перебор username.
	userSearchLimiter := ratelimit.New(20, time.Minute)

	mux.HandleFunc("POST /register", handlers.HandleRegister(d.users, d.jwt))
	mux.HandleFunc("POST /login", handlers.HandleLogin(d.users, d.jwt))
	mux.Handle("GET /me", authMW(handlers.HandleMe(d.users)))

	mux.Handle("GET /users/by-username/{username}",
		authMW(userSearchLimiter.Middleware(handlers.HandleFindUserByUsername(d.users))))

	mux.Handle("POST /chats", authMW(handlers.HandleCreateChat(d.chats, d.users)))
	mux.Handle("GET /chats", authMW(handlers.HandleGetMyChats(d.chats, d.users)))

	mux.Handle("POST /chats/{id}/messages", authMW(handlers.HandleSendMessage(d.chats, d.messages, d.hub)))
	mux.Handle("GET /chats/{id}/messages", authMW(handlers.HandleGetMessages(d.chats, d.messages)))

	mux.Handle("GET /ws", handlers.HandleWS(d.jwt, d.hub))

	return http.MaxBytesHandler(mux, maxBodyBytes)
}

func newHTTPServer(addr string, h http.Handler, t serverTimeouts) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           h,
		ReadHeaderTimeout: t.ReadHeader,
		ReadTimeout:       t.Read,
		WriteTimeout:      t.Write,
		IdleTimeout:       t.Idle,
	}
}
