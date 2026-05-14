package handlers

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/sapfirmoscow/chat-server-go/internal/auth"
)

func HandleWS(jwtManager *auth.Manager) http.HandlerFunc {
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

		log.Printf("ws connected: user=%s", userID)

		if err := echoLoop(r.Context(), conn, userID); err != nil {
			log.Printf("ws loop ended for user=%s: %v", userID, err)
			return
		}

		conn.Close(websocket.StatusNormalClosure, "")
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

func echoLoop(ctx context.Context, conn *websocket.Conn, userID string) error {
	for {

		msgType, data, err := conn.Read(ctx)
		if err != nil {
			return err
		}

		log.Printf("ws recv from %s: %s", userID, string(data))

		writeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err = conn.Write(writeCtx, msgType, data)
		cancel()
		if err != nil {
			return err
		}
	}
}
