package ws

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/coder/websocket"
)

// sendBufferSize — сколько сообщений может скопиться в очереди клиента,
// пока writePump не успевает их слать. Если переполнится — клиент считается
// мёртвым и дропается.
const (
	sendBufferSize = 32

	pingInterval = 30 * time.Second // как часто шлём ping
	pingTimeout  = 10 * time.Second // сколько ждём pong
	writeTimeout = 5 * time.Second  // таймаут на одну запись
)

type Client struct {
	UserID string
	conn   *websocket.Conn
	send   chan []byte
}

func NewClient(userID string, conn *websocket.Conn) *Client {
	return &Client{
		UserID: userID,
		conn:   conn,
		send:   make(chan []byte, sendBufferSize),
	}
}

func (c *Client) enqueue(msg []byte) error {
	select {
	case c.send <- msg:
		return nil
	default:
		return errors.New("send buffer full")
	}
}

// WritePump читает из канала send и пишет в websocket-соединение.
// Завершается, когда:
//   - канал send закрыт (Hub.Unregister) — нормальное завершение
//   - ctx отменён (PingPump детектит мёртвый коннект, или HTTP-запрос упал)
//   - conn.Write вернул ошибку (TCP-проблема)
func (c *Client) WritePump(ctx context.Context) {
	for {
		select {
		case msg, ok := <-c.send:
			if !ok {
				// канал закрыт Hub'ом — корректное завершение
				return
			}

			writeCtx, cancel := context.WithTimeout(ctx, writeTimeout)
			err := c.conn.Write(writeCtx, websocket.MessageText, msg)
			cancel()
			if err != nil {
				log.Printf("ws write error user=%s: %v", c.UserID, err)
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

func (c *Client) ReadPump(ctx context.Context) {
	for {
		_, _, err := c.conn.Read(ctx)
		if err != nil {
			log.Printf("ws read ended user=%s: %v", c.UserID, err)
			return
		}
	}
}

// PingPump периодически отправляет ping-фреймы и закрывает соединение,
// если pong не приходит за pingTimeout.
// Завершается при отмене ctx.
func (c *Client) PingPump(ctx context.Context) {
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// log.Printf("ws ping -> user=%s", c.UserID) // ← добавь
			pingCtx, cancel := context.WithTimeout(ctx, pingTimeout)
			err := c.conn.Ping(pingCtx)
			cancel()
			if err != nil {
				// log.Printf("ws ping failed user=%s: %v", c.UserID, err)
				// Закрываем conn — это разбудит ReadPump и WritePump
				// (они получат ошибку от своих conn.Read/conn.Write).
				_ = c.conn.CloseNow()
				return
			}
			// log.Printf("ws pong <- user=%s", c.UserID) // ← и сюда

		case <-ctx.Done():
			return
		}
	}
}
