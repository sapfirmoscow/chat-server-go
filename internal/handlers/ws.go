package handlers

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/coder/websocket"
	"github.com/sapfirmoscow/chat-server-go/internal/auth"
	wsPkg "github.com/sapfirmoscow/chat-server-go/internal/ws"
)

func HandleWS(jwtManager *auth.Manager, hub *wsPkg.Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		userID, err := extractUserIDFromSubprotocol(r, jwtManager)

		if err != nil {
			WriteError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			Subprotocols: []string{"access_token"},
		})
		if err != nil {
			log.Printf("ws accept error: %v", err)
			return
		}

		defer conn.CloseNow()

		client := wsPkg.NewClient(userID, conn)
		hub.Register(client)

		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()

		// Запускаем write и ping в отдельных горутинах.
		go client.WritePump(ctx)
		go client.PingPump(ctx)

		// ReadPump блокирует текущую горутину.
		// Когда он вернётся (клиент отключился или ping не прошёл),
		// cancel() через defer завершит остальные горутины.
		client.ReadPump(ctx)

		cancel() // явный cancel — на случай, если ReadPump вышел сам, а другим горутинам тоже пора
		hub.Unregister(client)

		log.Printf("ws disconnected: user=%s", userID)

	}
}

func extractUserIDFromSubprotocol(r *http.Request, jwtManager *auth.Manager) (string, error) {
	header := r.Header.Get("Sec-WebSocket-Protocol")
	if header == "" {
		return "", errors.New("missing subprotocol")
	}

	parts := strings.Split(header, ",")
	if len(parts) != 2 {
		return "", errors.New("invalid subprotocol format")
	}

	marker := strings.TrimSpace(parts[0])
	token := strings.TrimSpace(parts[1])

	if marker != "access_token" {
		return "", errors.New("unexpected subprotocol marker")
	}
	if token == "" {
		return "", errors.New("empty token")
	}

	userID, err := jwtManager.Verify(token)
	if err != nil {
		return "", err
	}

	return userID, nil
}
