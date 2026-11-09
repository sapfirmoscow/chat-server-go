package handlers

import (
	"strings"
	"testing"
)

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{"ok", "secret1", false},
		{"min length", "123456", false},
		{"too short", "12345", true},
		{"cyrillic counted in characters", "абв", true}, // 6 байт, но 3 символа
		{"cyrillic ok", "пароль", false},
		{"max bytes", strings.Repeat("a", 72), false},
		{"too long for bcrypt", strings.Repeat("a", 73), true},
		{"too long in bytes", strings.Repeat("ж", 37), true}, // 74 байта
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validatePassword(tt.password); (got != "") != tt.wantErr {
				t.Fatalf("validatePassword(%q) = %q, wantErr %v", tt.password, got, tt.wantErr)
			}
		})
	}
}

func TestValidateUsername(t *testing.T) {
	valid := []string{"alice", "bob_42", "a.b", "abc", strings.Repeat("a", 32), "123"}
	invalid := []string{
		"ab",                    // короче 3
		strings.Repeat("a", 33), // длиннее 32
		"алиса",                 // кириллица
		"аlice",                 // кириллическая "а" + латиница — подделка ника
		"al ice",                // пробел
		"al-ice",                // дефис
		"alice!",
		"Alice", // validateUsername ждёт нормализованное значение
		"",
	}
	for _, u := range valid {
		if msg := validateUsername(u); msg != "" {
			t.Errorf("validateUsername(%q) = %q, want valid", u, msg)
		}
	}
	for _, u := range invalid {
		if validateUsername(u) == "" {
			t.Errorf("validateUsername(%q) accepted, want error", u)
		}
	}
}

func TestNormalizeUsername(t *testing.T) {
	if got := normalizeUsername("  Alice.Smith "); got != "alice.smith" {
		t.Fatalf("normalizeUsername = %q, want %q", got, "alice.smith")
	}
}
