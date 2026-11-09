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
