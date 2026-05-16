package ws

import (
	"log"
	"sync"
)

// Hub держит реестр всех активных клиентов и умеет адресовать им сообщения
// по userID. Безопасен для конкурентного использования
type Hub struct {
	mu      sync.RWMutex
	clients map[string]map[*Client]struct{}
}

func NewHub() *Hub {
	return &Hub{
		clients: make(map[string]map[*Client]struct{}),
	}
}

// Register добавляет клиента в реестр. Один user может иметь несколько
// клиентов (телефон + десктоп).
func (h *Hub) Register(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	set, ok := h.clients[c.UserID]
	if !ok {
		set = make(map[*Client]struct{})
		h.clients[c.UserID] = set
	}
	set[c] = struct{}{}
	log.Printf("hub register user=%s total_conns=%d", c.UserID, len(set))

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
