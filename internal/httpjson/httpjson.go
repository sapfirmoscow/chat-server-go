// Package httpjson — запись JSON-ответов, общая для обработчиков и middleware.
//
// Вынесено в отдельный пакет, чтобы auth и ratelimit могли отвечать в том же
// формате, что и handlers, не импортируя handlers (это дало бы цикл импортов).
package httpjson

import (
	"encoding/json"
	"log"
	"net/http"
)

// ErrorResponse — тело всех ошибок API: {"error": "..."}.
type ErrorResponse struct {
	Error string `json:"error"`
}

// Write отправляет v как JSON с указанным статусом.
func Write(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("encode error: %v", err)
	}
}

// Error отправляет ошибку в формате ErrorResponse.
func Error(w http.ResponseWriter, status int, message string) {
	Write(w, status, ErrorResponse{Error: message})
}
