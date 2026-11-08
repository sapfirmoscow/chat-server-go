package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/sapfirmoscow/chat-server-go/internal/auth"
	"github.com/sapfirmoscow/chat-server-go/internal/models"
	"github.com/sapfirmoscow/chat-server-go/internal/storage"
)

type CreateChatRequest struct {
	MemberID string `json:"member_id"`
}

type ChatResponse struct {
	ID        string              `json:"id"`
	Members   []models.PublicUser `json:"members"`
	CreatedAt time.Time           `json:"created_at"`
}

type ChatsResponse struct {
	Chats []ChatResponse `json:"chats"`
}

func HandleCreateChat(chatStorage storage.Chats, userStorage storage.Users) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		currentUserID, ok := auth.GetUserID(r.Context())

		if !ok {
			WriteError(w, http.StatusInternalServerError, "Cant found userID in context")
			return
		}
		var req CreateChatRequest

		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()

		err := decoder.Decode(&req)
		if err != nil {
			WriteError(w, http.StatusBadRequest, "invalid json")
			return
		}

		//check id is empty
		if req.MemberID == "" {
			WriteError(w, http.StatusBadRequest, "member id is empty")
			return
		}

		//check is chat with yourself
		if req.MemberID == currentUserID {
			WriteError(w, http.StatusBadRequest, "can't create chat with yourself")
			return
		}

		//check that member exist
		_, err = userStorage.GetByID(r.Context(), req.MemberID)
		if errors.Is(err, storage.ErrNotFound) {
			WriteError(w, http.StatusNotFound, "member not found")
			return
		} else if err != nil {
			log.Printf("get member error: %v", err)
			WriteError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		// Поиск существующего чата и создание нового — одна атомарная операция
		// хранилища, поэтому параллельные запросы не создадут дубликат.
		newChat := &models.Chat{
			ID:        uuid.NewString(),
			MemberIDs: []string{currentUserID, req.MemberID},
			CreatedAt: time.Now().UTC(),
		}

		chat, created, err := chatStorage.GetOrCreateDirect(r.Context(), newChat)
		if err != nil {
			log.Printf("get or create chat error: %v", err)
			WriteError(w, http.StatusInternalServerError, "failed to create chat")
			return
		}

		status := http.StatusOK // чат уже существовал
		if created {
			status = http.StatusCreated
		}

		response := BuildChatResponse(r.Context(), chat, userStorage)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)

		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("encode error: %v", err)
		}

	}
}

func HandleGetMyChats(chatStorage storage.Chats, userStorage storage.Users) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		currentUserID, ok := auth.GetUserID(r.Context())

		if !ok {
			WriteError(w, http.StatusInternalServerError, "Cant found userID in context")
			return
		}

		chats, err := chatStorage.GetUserChats(r.Context(), currentUserID)
		if err != nil {
			log.Printf("get user chats error: %v", err)
			WriteError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		responses := make([]ChatResponse, 0, len(chats))

		for _, chat := range chats {
			responses = append(responses, BuildChatResponse(r.Context(), chat, userStorage))
		}

		chatResponse := ChatsResponse{
			Chats: responses,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		if err := json.NewEncoder(w).Encode(chatResponse); err != nil {
			log.Printf("encode error: %v", err)
		}
	}
}
