// Package ratelimit ограничивает частоту запросов к отдельным эндпоинтам.
package ratelimit

import (
	"math"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/sapfirmoscow/chat-server-go/internal/auth"
	"github.com/sapfirmoscow/chat-server-go/internal/httpjson"
)

// Limiter — счётчик запросов в фиксированном окне.
//
// Ключ — ID пользователя (если запрос прошёл auth.Middleware), иначе IP клиента.
// Хранилище in-memory: при нескольких репликах сервера лимит действует на каждую
// отдельно.
type Limiter struct {
	mu        sync.Mutex
	counters  map[string]*counter
	limit     int
	window    time.Duration
	lastSweep time.Time
	now       func() time.Time // подменяется в тестах
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
		now:      time.Now,
	}
}

// allow учитывает запрос с ключом key. Если лимит исчерпан, возвращает false
// и время до начала следующего окна.
func (l *Limiter) allow(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	l.sweep(now)

	c, ok := l.counters[key]
	if !ok || now.Sub(c.windowStart) >= l.window {
		l.counters[key] = &counter{count: 1, windowStart: now}
		return true, 0
	}

	if c.count >= l.limit {
		return false, c.windowStart.Add(l.window).Sub(now)
	}

	c.count++
	return true, 0
}

// sweep удаляет записи с истёкшим окном, чтобы map не рос неограниченно.
// Полный обход делается не чаще раза за окно: иначе поток запросов с разных
// IP заставлял бы обходить всю map на каждом запросе.
// Вызывается под захваченным mu.
func (l *Limiter) sweep(now time.Time) {
	if now.Sub(l.lastSweep) < l.window {
		return
	}
	l.lastSweep = now
	for key, c := range l.counters {
		if now.Sub(c.windowStart) >= l.window {
			delete(l.counters, key)
		}
	}
}

// Middleware отклоняет запросы сверх лимита с кодом 429 и заголовком Retry-After.
//
// Если выше по цепочке отработал auth.Middleware, ключом служит ID пользователя,
// иначе — IP клиента (без порта: у каждого TCP-соединения свой порт, и лимит по
// RemoteAddr целиком обходился бы простым переподключением).
//
// IP берётся из RemoteAddr. За обратным прокси это адрес прокси, и лимит станет
// общим для всех клиентов — тогда нужно доверять X-Forwarded-For от прокси.
func (l *Limiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key, ok := auth.GetUserID(r.Context())
		if !ok {
			key = "ip:" + clientIP(r)
		}

		if allowed, retryAfter := l.allow(key); !allowed {
			w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(retryAfter.Seconds()))))
			httpjson.Error(w, http.StatusTooManyRequests, "too many requests")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
