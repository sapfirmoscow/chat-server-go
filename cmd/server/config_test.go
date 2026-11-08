package main

import (
	"strings"
	"testing"
)

const testSecret = "0123456789abcdef0123456789abcdef" // 32 байта

func env(vars map[string]string) func(string) string {
	return func(key string) string { return vars[key] }
}

func TestLoadConfigDefaults(t *testing.T) {
	cfg, err := loadConfig(env(map[string]string{"JWT_SECRET": testSecret}))
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if cfg.Addr != ":8080" || cfg.StorageDriver != "memory" || cfg.DatabaseURL != "" || cfg.JWTSecret != testSecret {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
}

func TestLoadConfigFromEnv(t *testing.T) {
	cfg, err := loadConfig(env(map[string]string{
		"JWT_SECRET":     testSecret,
		"PORT":           "9090",
		"STORAGE_DRIVER": "postgres",
		"DATABASE_URL":   "postgres://localhost/chat",
	}))
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if cfg.Addr != ":9090" || cfg.StorageDriver != "postgres" || cfg.DatabaseURL != "postgres://localhost/chat" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}

func TestLoadConfigErrors(t *testing.T) {
	tests := []struct {
		name    string
		vars    map[string]string
		wantErr string
	}{
		{"missing secret", map[string]string{}, "JWT_SECRET is required"},
		{"short secret", map[string]string{"JWT_SECRET": "short"}, "at least 32 bytes"},
		{"bad port", map[string]string{"JWT_SECRET": testSecret, "PORT": "http"}, "PORT must be"},
		{"port out of range", map[string]string{"JWT_SECRET": testSecret, "PORT": "70000"}, "PORT must be"},
		{"all errors at once", map[string]string{"PORT": "0"}, "PORT must be"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := loadConfig(env(tt.vars))
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("got %v, want error containing %q", err, tt.wantErr)
			}
		})
	}
}
