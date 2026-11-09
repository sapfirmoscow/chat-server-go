// Package ratelimit ограничивает частоту запросов к отдельным эндпоинтам.
package ratelimit

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/sapfirmoscow/chat-server-go/internal/auth"
	"github.com/sapfirmoscow/chat-server-go/internal/httpjson"
)

// Limiter — счётчик запросов в фиксированном окне.
//
// Ключ — ID пользователя (если запрос прошёл auth.Middleware), иначе адрес клиента.
// Хранилище in-memory, как и остальные хранилища сервера.
type Limiter struct {
	mu       sync.Mutex
	counters map[string]*counter
	limit    int
	window   time.Duration
}

type counter struct {
	count       int
	windowStart time.Time
}

// New создаёт лимитер, разрешающий не более limit запросов за window.
func New(limit int, window time.Duration) *Limiter {
	return &Limiter{
		counters: make(map[string]*counter),
		limit:    limit,
		window:   window,
	}
}

func (l *Limiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	c, ok := l.counters[key]

	if !ok || now.Sub(c.windowStart) >= l.window {
		l.sweep(now)
		l.counters[key] = &counter{count: 1, windowStart: now}
		return true
	}

	if c.count >= l.limit {
		return false
	}

	c.count++
	return true
}

// sweep удаляет записи с истёкшим окном, чтобы map не рос неограниченно.
// Вызывается уже под захваченным mu.
func (l *Limiter) sweep(now time.Time) {
	for key, c := range l.counters {
		if now.Sub(c.windowStart) >= l.window {
			delete(l.counters, key)
		}
	}
}

// Middleware отклоняет запросы сверх лимита с кодом 429.
//
// Ожидает, что выше по цепочке уже отработал auth.Middleware — тогда ключом
// служит ID пользователя. Иначе откатывается на r.RemoteAddr.
func (l *Limiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key, ok := auth.GetUserID(r.Context())
		if !ok {
			key = r.RemoteAddr
		}

		if !l.allow(key) {
			w.Header().Set("Retry-After", strconv.Itoa(int(l.window.Seconds())))
			httpjson.Error(w, http.StatusTooManyRequests, "too many requests")
			return
		}

		next.ServeHTTP(w, r)
	})
}
