// Package storagetest — общий набор тестов, которому должна соответствовать
// любая реализация интерфейсов storage (memory, SQLite, PostgreSQL).
package storagetest

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sapfirmoscow/chat-server-go/internal/models"
	"github.com/sapfirmoscow/chat-server-go/internal/storage"
)

// Stores — набор хранилищ одной реализации.
type Stores struct {
	Users    storage.Users
	Chats    storage.Chats
	Messages storage.Messages
}

// Factory возвращает пустые хранилища; очистку регистрирует через t.Cleanup.
type Factory func(t *testing.T) Stores

// Run прогоняет все проверки на хранилищах, созданных newStores.
func Run(t *testing.T, newStores Factory) {
	tests := []struct {
		name string
		fn   func(t *testing.T, s Stores)
	}{
		{"UsersAddAndGet", testUsersAddAndGet},
		{"UsersUsernameTaken", testUsersUsernameTaken},
		{"UsersNotFound", testUsersNotFound},
		{"ChatsGetOrCreateDirect", testChatsGetOrCreateDirect},
		{"ChatsGetOrCreateDirectConcurrent", testChatsGetOrCreateDirectConcurrent},
		{"ChatsInvalid", testChatsInvalid},
		{"ChatsNotFound", testChatsNotFound},
		{"ChatsUserChatsOrder", testChatsUserChatsOrder},
		{"MessagesHistory", testMessagesHistory},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.fn(t, newStores(t))
		})
	}
}

// ts — время с точностью до микросекунд: столько хранит PostgreSQL.
func ts(offset time.Duration) time.Time {
	return time.Date(2026, 5, 16, 10, 0, 0, 0, time.UTC).Add(offset)
}

func addUser(t *testing.T, s Stores, username string) *models.User {
	t.Helper()
	u := &models.User{
		ID:           uuid.NewString(),
		Username:     username,
		PasswordHash: "hash-" + username,
		CreatedAt:    ts(0),
	}
	if err := s.Users.Add(context.Background(), u); err != nil {
		t.Fatalf("add user %q: %v", username, err)
	}
	return u
}

func newDirect(a, b string, createdAt time.Time) *models.Chat {
	return &models.Chat{ID: uuid.NewString(), MemberIDs: []string{a, b}, CreatedAt: createdAt}
}

func testUsersAddAndGet(t *testing.T, s Stores) {
	ctx := context.Background()
	want := addUser(t, s, "alice")

	got, err := s.Users.GetByID(ctx, want.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	assertUser(t, got, want)

	got, err = s.Users.GetByUsername(ctx, "ALICE")
	if err != nil {
		t.Fatalf("GetByUsername (other case): %v", err)
	}
	assertUser(t, got, want)
}

func testUsersUsernameTaken(t *testing.T, s Stores) {
	addUser(t, s, "bob")

	dup := &models.User{ID: uuid.NewString(), Username: "Bob", PasswordHash: "x", CreatedAt: ts(0)}
	err := s.Users.Add(context.Background(), dup)
	if !errors.Is(err, storage.ErrUsernameTaken) {
		t.Fatalf("Add duplicate: got %v, want ErrUsernameTaken", err)
	}
}

func testUsersNotFound(t *testing.T, s Stores) {
	ctx := context.Background()
	if _, err := s.Users.GetByID(ctx, uuid.NewString()); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("GetByID: got %v, want ErrNotFound", err)
	}
	if _, err := s.Users.GetByUsername(ctx, "nobody"); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("GetByUsername: got %v, want ErrNotFound", err)
	}
}

func testChatsGetOrCreateDirect(t *testing.T, s Stores) {
	ctx := context.Background()
	a, b := addUser(t, s, "alice"), addUser(t, s, "bob")

	first := newDirect(a.ID, b.ID, ts(time.Minute))
	got, created, err := s.Chats.GetOrCreateDirect(ctx, first)
	if err != nil || !created {
		t.Fatalf("first GetOrCreateDirect: created=%v err=%v", created, err)
	}
	assertChat(t, got, first)

	// Тот же чат, участники в обратном порядке.
	got, created, err = s.Chats.GetOrCreateDirect(ctx, newDirect(b.ID, a.ID, ts(2*time.Minute)))
	if err != nil {
		t.Fatalf("second GetOrCreateDirect: %v", err)
	}
	if created {
		t.Fatal("second GetOrCreateDirect: created a duplicate chat")
	}
	assertChat(t, got, first)

	got, err = s.Chats.GetByID(ctx, first.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	assertChat(t, got, first)
}

func testChatsGetOrCreateDirectConcurrent(t *testing.T, s Stores) {
	ctx := context.Background()
	a, b := addUser(t, s, "alice"), addUser(t, s, "bob")

	const n = 10
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		ids     = map[string]bool{}
		created int
		errs    []error
	)
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			chat := newDirect(a.ID, b.ID, ts(time.Duration(i)*time.Second))
			if i%2 == 1 {
				chat.MemberIDs = []string{b.ID, a.ID}
			}
			got, c, err := s.Chats.GetOrCreateDirect(ctx, chat)

			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, err)
				return
			}
			ids[got.ID] = true
			if c {
				created++
			}
		}()
	}
	wg.Wait()

	if len(errs) > 0 {
		t.Fatalf("errors: %v", errs)
	}
	if len(ids) != 1 || created != 1 {
		t.Fatalf("got %d distinct chats, %d created; want 1 and 1", len(ids), created)
	}
}

func testChatsInvalid(t *testing.T, s Stores) {
	ctx := context.Background()
	a := addUser(t, s, "alice")

	cases := map[string]*models.Chat{
		"one member":  {ID: uuid.NewString(), MemberIDs: []string{a.ID}, CreatedAt: ts(0)},
		"with itself": newDirect(a.ID, a.ID, ts(0)),
	}
	for name, chat := range cases {
		if _, _, err := s.Chats.GetOrCreateDirect(ctx, chat); !errors.Is(err, storage.ErrInvalidChat) {
			t.Errorf("%s: got %v, want ErrInvalidChat", name, err)
		}
	}
}

func testChatsNotFound(t *testing.T, s Stores) {
	if _, err := s.Chats.GetByID(context.Background(), "no-such-chat"); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("GetByID: got %v, want ErrNotFound", err)
	}
}

func testChatsUserChatsOrder(t *testing.T, s Stores) {
	ctx := context.Background()
	a, b, c, d := addUser(t, s, "alice"), addUser(t, s, "bob"), addUser(t, s, "carol"), addUser(t, s, "dave")

	older := newDirect(a.ID, b.ID, ts(time.Minute))
	newer := newDirect(c.ID, a.ID, ts(time.Hour))
	other := newDirect(c.ID, d.ID, ts(2*time.Hour))
	for _, chat := range []*models.Chat{older, newer, other} {
		if _, _, err := s.Chats.GetOrCreateDirect(ctx, chat); err != nil {
			t.Fatalf("create chat: %v", err)
		}
	}

	got, err := s.Chats.GetUserChats(ctx, a.ID)
	if err != nil {
		t.Fatalf("GetUserChats: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d chats, want 2", len(got))
	}
	assertChat(t, got[0], newer)
	assertChat(t, got[1], older)

	got, err = s.Chats.GetUserChats(ctx, uuid.NewString())
	if err != nil {
		t.Fatalf("GetUserChats for user without chats: %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Fatalf("got %v, want empty non-nil slice", got)
	}
}

func testMessagesHistory(t *testing.T, s Stores) {
	ctx := context.Background()
	a, b, c := addUser(t, s, "alice"), addUser(t, s, "bob"), addUser(t, s, "carol")

	chat := newDirect(a.ID, b.ID, ts(0))
	other := newDirect(a.ID, c.ID, ts(0))
	for _, ch := range []*models.Chat{chat, other} {
		if _, _, err := s.Chats.GetOrCreateDirect(ctx, ch); err != nil {
			t.Fatalf("create chat: %v", err)
		}
	}

	empty, err := s.Messages.GetByChat(ctx, chat.ID, 10)
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("empty chat: got %v, %v; want empty non-nil slice", empty, err)
	}

	var sent []*models.Message
	for i := range 5 {
		// Одинаковое время у всех: порядок должен определяться порядком вставки.
		msg := &models.Message{
			ID:        uuid.NewString(),
			ChatID:    chat.ID,
			SenderID:  []string{a.ID, b.ID}[i%2],
			Text:      fmt.Sprintf("msg %d", i),
			CreatedAt: ts(time.Minute),
		}
		if err := s.Messages.Add(ctx, msg); err != nil {
			t.Fatalf("Add: %v", err)
		}
		sent = append(sent, msg)
	}
	noise := &models.Message{ID: uuid.NewString(), ChatID: other.ID, SenderID: c.ID, Text: "noise", CreatedAt: ts(time.Minute)}
	if err := s.Messages.Add(ctx, noise); err != nil {
		t.Fatalf("Add noise: %v", err)
	}

	got, err := s.Messages.GetByChat(ctx, chat.ID, 3)
	if err != nil {
		t.Fatalf("GetByChat: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d messages, want 3", len(got))
	}
	for i, msg := range got {
		assertMessage(t, msg, sent[2+i])
	}

	got, err = s.Messages.GetByChat(ctx, chat.ID, 100)
	if err != nil || len(got) != 5 {
		t.Fatalf("GetByChat(limit 100): got %d messages, err %v; want 5", len(got), err)
	}
}

func assertUser(t *testing.T, got, want *models.User) {
	t.Helper()
	if got.ID != want.ID || got.Username != want.Username ||
		got.PasswordHash != want.PasswordHash || !got.CreatedAt.Equal(want.CreatedAt) {
		t.Fatalf("user mismatch:\n got  %+v\n want %+v", got, want)
	}
	if got.CreatedAt.Location() != time.UTC {
		t.Errorf("user CreatedAt location = %v, want UTC", got.CreatedAt.Location())
	}
}

func assertChat(t *testing.T, got, want *models.Chat) {
	t.Helper()
	if got.ID != want.ID || !got.CreatedAt.Equal(want.CreatedAt) || fmt.Sprint(got.MemberIDs) != fmt.Sprint(want.MemberIDs) {
		t.Fatalf("chat mismatch:\n got  %+v\n want %+v", got, want)
	}
	if got.CreatedAt.Location() != time.UTC {
		t.Errorf("chat CreatedAt location = %v, want UTC", got.CreatedAt.Location())
	}
}

func assertMessage(t *testing.T, got, want *models.Message) {
	t.Helper()
	if got.ID != want.ID || got.ChatID != want.ChatID || got.SenderID != want.SenderID ||
		got.Text != want.Text || !got.CreatedAt.Equal(want.CreatedAt) {
		t.Fatalf("message mismatch:\n got  %+v\n want %+v", got, want)
	}
	if got.CreatedAt.Location() != time.UTC {
		t.Errorf("message CreatedAt location = %v, want UTC", got.CreatedAt.Location())
	}
}
