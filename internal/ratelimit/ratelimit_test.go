package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time          { return c.t }
func (c *fakeClock) advance(d time.Duration) { c.t = c.t.Add(d) }

func newTestLimiter(limit int, window time.Duration) (*Limiter, *fakeClock) {
	clock := &fakeClock{t: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	l := New(limit, window)
	l.now = clock.now
	return l, clock
}

func TestAllowWithinWindow(t *testing.T) {
	l, clock := newTestLimiter(3, time.Minute)

	for i := range 3 {
		if ok, _ := l.allow("a"); !ok {
			t.Fatalf("request %d rejected, want allowed", i+1)
		}
	}

	clock.advance(20 * time.Second)
	ok, retryAfter := l.allow("a")
	if ok {
		t.Fatal("4th request allowed, want rejected")
	}
	if retryAfter != 40*time.Second {
		t.Fatalf("retryAfter = %v, want 40s (rest of the window)", retryAfter)
	}

	// Другой ключ считается отдельно.
	if ok, _ := l.allow("b"); !ok {
		t.Fatal("other key rejected")
	}

	// Новое окно — счётчик сброшен.
	clock.advance(40 * time.Second)
	if ok, _ := l.allow("a"); !ok {
		t.Fatal("request in the next window rejected")
	}
}

func TestSweepRemovesExpiredKeys(t *testing.T) {
	l, clock := newTestLimiter(1, time.Minute)

	for _, key := range []string{"a", "b", "c"} {
		l.allow(key)
	}
	clock.advance(time.Minute)
	l.allow("d")

	if len(l.counters) != 1 {
		t.Fatalf("counters = %d, want 1 (expired keys swept)", len(l.counters))
	}
}

// Разные порты одного IP — один и тот же клиент.
func TestMiddlewareKeysByIPWithoutPort(t *testing.T) {
	l, _ := newTestLimiter(2, time.Minute)
	h := l.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	codes := make([]int, 0, 3)
	for _, addr := range []string{"203.0.113.7:1111", "203.0.113.7:2222", "203.0.113.7:3333"} {
		req := httptest.NewRequest(http.MethodPost, "/login", nil)
		req.RemoteAddr = addr
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		codes = append(codes, rec.Code)
	}

	if codes[0] != http.StatusOK || codes[1] != http.StatusOK || codes[2] != http.StatusTooManyRequests {
		t.Fatalf("codes = %v, want [200 200 429]", codes)
	}

	// Другой IP не затронут.
	req := httptest.NewRequest(http.MethodPost, "/login", nil)
	req.RemoteAddr = "198.51.100.1:1111"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("other IP: code %d, want 200", rec.Code)
	}
}

func TestMiddlewareRetryAfterAndJSON(t *testing.T) {
	l, clock := newTestLimiter(1, time.Minute)
	h := l.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	serve := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/login", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	serve()
	clock.advance(15500 * time.Millisecond)
	rec := serve()

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("code = %d, want 429", rec.Code)
	}
	if got := rec.Header().Get("Retry-After"); got != "45" { // 44.5s округляется вверх
		t.Fatalf("Retry-After = %q, want 45", got)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}
}
