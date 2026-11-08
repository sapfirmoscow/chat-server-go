package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/sapfirmoscow/chat-server-go/internal/auth"
	"github.com/sapfirmoscow/chat-server-go/internal/storage/memory"
	"github.com/sapfirmoscow/chat-server-go/internal/ws"
)

type testServer struct {
	*httptest.Server
	hub *ws.Hub
}

func newTestServer(t *testing.T, timeouts serverTimeouts) *testServer {
	t.Helper()
	hub := ws.NewHub()
	router := newRouter(deps{
		users:    memory.NewUsers(),
		chats:    memory.NewChats(),
		messages: memory.NewMessages(),
		jwt:      auth.NewManager(testSecret, time.Hour),
		hub:      hub,
	})

	srv := httptest.NewUnstartedServer(router)
	srv.Config = newHTTPServer("", router, timeouts)
	srv.Start()
	t.Cleanup(srv.Close)
	return &testServer{Server: srv, hub: hub}
}

// do отправляет JSON-запрос и декодирует ответ в out (если out != nil).
func (s *testServer) do(t *testing.T, method, path, token string, body any, out any) int {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}
	req, err := http.NewRequest(method, s.URL+path, &buf)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := s.Client().Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			t.Fatalf("%s %s: decode response: %v", method, path, err)
		}
	}
	return resp.StatusCode
}

type authResp struct {
	Token string `json:"token"`
	User  struct {
		ID string `json:"id"`
	} `json:"user"`
}

func (s *testServer) register(t *testing.T, username string) authResp {
	t.Helper()
	var out authResp
	status := s.do(t, http.MethodPost, "/register", "", map[string]string{"username": username, "password": "secret1"}, &out)
	if status != http.StatusCreated {
		t.Fatalf("register %s: status %d", username, status)
	}
	return out
}

func TestRequestBodyTooLarge(t *testing.T) {
	s := newTestServer(t, defaultTimeouts)

	body := `{"username":"` + strings.Repeat("a", maxBodyBytes) + `","password":"secret1"}`
	resp, err := s.Client().Post(s.URL+"/register", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", resp.StatusCode)
	}
}

func TestInvalidJSONStillBadRequest(t *testing.T) {
	s := newTestServer(t, defaultTimeouts)

	resp, err := s.Client().Post(s.URL+"/register", "application/json", strings.NewReader(`{"username":`))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

// WebSocket должен жить дольше ReadTimeout/WriteTimeout сервера. Эти таймауты
// ставят дедлайны на TCP-соединение; net/http сбрасывает их при hijack,
// и тест страхует от регрессии (например, при смене библиотеки WebSocket).
func TestWebSocketOutlivesServerTimeouts(t *testing.T) {
	const short = 200 * time.Millisecond
	s := newTestServer(t, serverTimeouts{ReadHeader: short, Read: short, Write: short, Idle: short})

	alice, bob := s.register(t, "alice"), s.register(t, "bob")

	var chat struct {
		ID string `json:"id"`
	}
	if status := s.do(t, http.MethodPost, "/chats", alice.Token, map[string]string{"member_id": bob.User.ID}, &chat); status != http.StatusCreated {
		t.Fatalf("create chat: status %d", status)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(s.URL, "http")+"/ws", &websocket.DialOptions{
		Subprotocols: []string{"access_token", bob.Token},
	})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.CloseNow()

	time.Sleep(3 * short) // дольше любого из серверных таймаутов

	if status := s.do(t, http.MethodPost, "/chats/"+chat.ID+"/messages", alice.Token, map[string]string{"text": "hi"}, nil); status != http.StatusCreated {
		t.Fatalf("send message: status %d", status)
	}

	readCtx, readCancel := context.WithTimeout(ctx, 2*time.Second)
	defer readCancel()
	_, data, err := conn.Read(readCtx)
	if err != nil {
		t.Fatalf("websocket closed after server timeouts: %v", err)
	}

	var event struct {
		Type string `json:"type"`
		Data struct {
			Text string `json:"text"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &event); err != nil {
		t.Fatalf("decode event: %v", err)
	}
	if event.Type != "message.new" || event.Data.Text != "hi" {
		t.Fatalf("unexpected event: %s", data)
	}
}

func (s *testServer) dialWS(t *testing.T, ctx context.Context, token string) *websocket.Conn {
	t.Helper()
	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(s.URL, "http")+"/ws", &websocket.DialOptions{
		Subprotocols: []string{"access_token", token},
	})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.CloseNow() })
	return conn
}

// При остановке сервера клиенты получают close-фрейм 1001 (going away),
// а новые подключения сразу закрываются тем же кодом.
func TestHubShutdownClosesWebSockets(t *testing.T) {
	s := newTestServer(t, defaultTimeouts)
	alice := s.register(t, "alice")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn := s.dialWS(t, ctx, alice.Token)

	// Read обязателен: он обрабатывает входящий close-фрейм и отвечает на него.
	readErr := make(chan error, 1)
	go func() {
		_, _, err := conn.Read(ctx)
		readErr <- err
	}()

	// Dial возвращается раньше, чем сервер зарегистрирует клиента в хабе.
	for s.hub.Connections() == 0 {
		if ctx.Err() != nil {
			t.Fatal("client was not registered in hub")
		}
		time.Sleep(5 * time.Millisecond)
	}

	shutdownDone := make(chan struct{})
	go func() {
		s.hub.Shutdown(ctx)
		close(shutdownDone)
	}()

	select {
	case err := <-readErr:
		if status := websocket.CloseStatus(err); status != websocket.StatusGoingAway {
			t.Fatalf("close status = %v (err %v), want StatusGoingAway", status, err)
		}
	case <-ctx.Done():
		t.Fatal("connection was not closed on shutdown")
	}

	select {
	case <-shutdownDone:
	case <-ctx.Done():
		t.Fatal("hub.Shutdown did not return")
	}

	late := s.dialWS(t, ctx, alice.Token)
	_, _, err := late.Read(ctx)
	if status := websocket.CloseStatus(err); status != websocket.StatusGoingAway {
		t.Fatalf("connection after shutdown: close status = %v (err %v), want StatusGoingAway", status, err)
	}
}
