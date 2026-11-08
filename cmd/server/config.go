package main

import (
	"errors"
	"fmt"
	"strconv"
)

// minJWTSecretLen — минимальная длина секрета HS256 в байтах (256 бит).
const minJWTSecretLen = 32

// config — настройки сервера из переменных окружения.
//
//	JWT_SECRET     — обязательный, не короче 32 байт
//	PORT           — порт HTTP-сервера, по умолчанию 8080
//	STORAGE_DRIVER — memory | sqlite | postgres, по умолчанию memory
//	DATABASE_URL   — путь к файлу SQLite или URL PostgreSQL
type config struct {
	Addr          string
	JWTSecret     string
	StorageDriver string
	DatabaseURL   string
}

// loadConfig читает настройки через getenv (os.Getenv в проде, заглушка в тестах).
func loadConfig(getenv func(string) string) (config, error) {
	cfg := config{
		JWTSecret:     getenv("JWT_SECRET"),
		StorageDriver: getenv("STORAGE_DRIVER"),
		DatabaseURL:   getenv("DATABASE_URL"),
	}

	var errs []error

	switch {
	case cfg.JWTSecret == "":
		errs = append(errs, errors.New("JWT_SECRET is required (generate one with: openssl rand -hex 32)"))
	case len(cfg.JWTSecret) < minJWTSecretLen:
		errs = append(errs, fmt.Errorf("JWT_SECRET must be at least %d bytes, got %d", minJWTSecretLen, len(cfg.JWTSecret)))
	}

	port := getenv("PORT")
	if port == "" {
		port = "8080"
	}
	if n, err := strconv.Atoi(port); err != nil || n < 1 || n > 65535 {
		errs = append(errs, fmt.Errorf("PORT must be a number in 1..65535, got %q", port))
	}
	cfg.Addr = ":" + port

	if cfg.StorageDriver == "" {
		cfg.StorageDriver = "memory"
	}

	if err := errors.Join(errs...); err != nil {
		return config{}, err
	}
	return cfg, nil
}
