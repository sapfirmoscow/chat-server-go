package ws

import (
	"context"
	"log"
	"sync"

	"github.com/coder/websocket"
)

// Hub держит реестр всех активных клиентов и умеет адресовать им сообщения
// по userID. Безопасен для конкурентного использования
type Hub struct {
	mu      sync.RWMutex
	clients map[string]map[*Client]struct{}
	closed  bool // после Shutdown новые клиенты не принимаются
}

func NewHub() *Hub {
	return &Hub{
		clients: make(map[string]map[*Client]struct{}),
	}
}

// Register добавляет клиента в реестр. Один user может иметь несколько
// клиентов (телефон + десктоп). Возвращает false, если хаб уже остановлен:
// тогда вызывающий должен закрыть соединение сам.
func (h *Hub) Register(c *Client) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.closed {
		return false
	}

	set, ok := h.clients[c.UserID]
	if !ok {
		set = make(map[*Client]struct{})
		h.clients[c.UserID] = set
	}
	set[c] = struct{}{}
	log.Printf("hub register user=%s total_conns=%d", c.UserID, len(set))
	return true
}

// Unregister удаляет клиента из реестра и закрывает его канал send.
// Закрытие канала здесь — потому что Hub единственный, кто в него пишет
// (см. SendToUser), и закрывать должен тот, кто пишет.
func (h *Hub) Unregister(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	set, ok := h.clients[c.UserID]
	if !ok {
		return
	}
	if _, exist := set[c]; !exist {
		return
	}

	delete(set, c)
	close(c.send)

	if len(set) == 0 {
		delete(h.clients, c.UserID)
	}
	log.Printf("hub unregister user=%s remaining_conns=%d", c.UserID, len(set))
}

// SendToUser отправляет сообщение всем активным соединениям пользователя.
// Если у юзера 0 соединений (offline) — ничего не происходит.
// Если у какого-то клиента буфер забит — клиент считается мёртвым,
// его надо дропнуть (но не в этой же горутине под мьютексом — собираем
// список и дропаем после).
func (h *Hub) SendToUser(userID string, msg []byte) {
	h.mu.RLock()
	set, ok := h.clients[userID]
	if !ok {
		h.mu.RUnlock()
		return
	}

	var dead []*Client
	for c := range set {
		if err := c.enqueue(msg); err != nil {
			dead = append(dead, c)
		}
	}
	h.mu.RUnlock()

	for _, c := range dead {
		log.Printf("hub dropping slow client user=%s", c.UserID)

		h.Unregister(c)
		_ = c.conn.CloseNow()
	}
}

// Connections возвращает число активных соединений по всем пользователям.
func (h *Hub) Connections() int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	n := 0
	for _, set := range h.clients {
		n += len(set)
	}
	return n
}

// Shutdown перестаёт принимать новых клиентов и закрывает все активные
// соединения с кодом 1001 (going away), чтобы клиенты знали, что пора
// переподключиться. Ждёт завершения close-handshake или отмены ctx.
func (h *Hub) Shutdown(ctx context.Context) {
	h.mu.Lock()
	h.closed = true
	var all []*Client
	for _, set := range h.clients {
		for c := range set {
			all = append(all, c)
		}
	}
	h.mu.Unlock()

	if len(all) == 0 {
		return
	}
	log.Printf("hub shutdown: closing %d connections", len(all))

	// Close блокируется до ответа клиента (до 5 с), поэтому закрываем параллельно
	// и вне мьютекса: обработчики соединений сами вызовут Unregister.
	var wg sync.WaitGroup
	for _, c := range all {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = c.conn.Close(websocket.StatusGoingAway, "server shutting down")
		}()
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-ctx.Done():
		for _, c := range all {
			_ = c.conn.CloseNow()
		}
	}
}
