package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/sapfirmoscow/chat-server-go/internal/models"
	"github.com/sapfirmoscow/chat-server-go/internal/storage"
)

type ErrorResponse struct {
	Error string `json:"error"`
}

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

func WriteError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(ErrorResponse{Error: message}); err != nil {
		log.Printf("encode error: %v", err)
	}
}
