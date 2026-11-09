package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/sapfirmoscow/chat-server-go/internal/httpjson"
	"github.com/sapfirmoscow/chat-server-go/internal/models"
	"github.com/sapfirmoscow/chat-server-go/internal/storage"
)

func BuildChatResponse(ctx context.Context, chat *models.Chat, userStorage storage.Users) ChatResponse {
	members := make([]models.PublicUser, 0, len(chat.MemberIDs))

	for _, id := range chat.MemberIDs {
		user, err := userStorage.GetByID(ctx, id)
		if err != nil {
			if !errors.Is(err, storage.ErrNotFound) {
				log.Printf("get chat member error: %v", err)
			}
			continue
		}
		members = append(members, user.ToPublic())
	}

	return ChatResponse{
		ID:        chat.ID,
		Members:   members,
		CreatedAt: chat.CreatedAt,
	}
}

// WriteError отправляет ошибку в едином формате {"error": "..."}.
func WriteError(w http.ResponseWriter, status int, message string) {
	httpjson.Error(w, status, message)
}

// DecodeJSON читает JSON-тело запроса в dst, запрещая неизвестные поля.
// При ошибке сам пишет ответ (413 для слишком большого тела, иначе 400)
// и возвращает false.
func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(dst); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			WriteError(w, http.StatusRequestEntityTooLarge, "request body too large")
			return false
		}
		WriteError(w, http.StatusBadRequest, "invalid json")
		return false
	}
	return true
}
