package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sapfirmoscow/chat-server-go/internal/auth"
	"github.com/sapfirmoscow/chat-server-go/internal/models"
	"github.com/sapfirmoscow/chat-server-go/internal/storage"
	wsPkg "github.com/sapfirmoscow/chat-server-go/internal/ws"
)

type SendMessageRequest struct {
	Text string `json:"text"`
}

type MessagesResponse struct {
	Messages []models.PublicMessage `json:"messages"`
}

func HandleSendMessage(chats *storage.Chats, messages *storage.Messages, hub *wsPkg.Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		chat, senderID, ok := resolveChatAccess(w, r, chats)
		if !ok {
			return
		}

		var req SendMessageRequest

		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()

		err := decoder.Decode(&req)
		if err != nil {
			WriteError(w, http.StatusBadRequest, "invalid json")
			return
		}

		normalizedText := strings.TrimSpace(req.Text)
		if len(normalizedText) == 0 {
			WriteError(w, http.StatusBadRequest, "text is empty")
			return
		}
		if len(normalizedText) > 2000 {
			WriteError(w, http.StatusBadRequest, "text is too long")
			return
		}

		message := models.Message{
			ID:        uuid.NewString(),
			ChatID:    chat.ID,
			SenderID:  senderID,
			Text:      normalizedText,
			CreatedAt: time.Now().UTC(),
		}

		messages.Add(&message)

		// рассылка через WebSocket участникам чата
		payload, err := wsPkg.Marshal(wsPkg.EventMessageNew, message.ToPublic())
		if err != nil {
			log.Printf("ws marshal error: %v", err)
		} else {
			for _, uid := range chat.MemberIDs {
				hub.SendToUser(uid, payload)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)

		if err := json.NewEncoder(w).Encode(message.ToPublic()); err != nil {
			log.Printf("encode error: %v", err)
		}
	}
}

func HandleGetMessages(chats *storage.Chats, messages *storage.Messages) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		chat, _, ok := resolveChatAccess(w, r, chats)
		if !ok {
			return
		}

		limit := 50
		if s := r.URL.Query().Get("limit"); s != "" {
			n, err := strconv.Atoi(s)
			if err != nil || n <= 0 {
				WriteError(w, http.StatusBadRequest, "invalid limit")
				return
			}
			if n > 100 {
				n = 100
			}
			limit = n
		}

		msgs := messages.GetByChat(chat.ID, limit)
		publicMessages := make([]models.PublicMessage, 0, len(msgs))

		for _, msg := range msgs {
			publicMessages = append(publicMessages, msg.ToPublic())
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		response := MessagesResponse{
			Messages: publicMessages,
		}

		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("encode error: %v", err)
		}

	}
}

func resolveChatAccess(
	w http.ResponseWriter,
	r *http.Request,
	chats *storage.Chats,
) (*models.Chat, string, bool) {
	userID, ok := auth.GetUserID(r.Context())

	if !ok {
		WriteError(w, http.StatusInternalServerError, "Cant found userID in context")
		return nil, "", false
	}

	chatID := r.PathValue("id")

	if chatID == "" {
		WriteError(w, http.StatusBadRequest, "chat id is empty")
		return nil, "", false
	}

	chat, ok := chats.GetByID(chatID)
	if !ok {
		WriteError(w, http.StatusNotFound, "chat not found")
		return nil, "", false
	}
	if !chat.HasMember(userID) {
		WriteError(w, http.StatusForbidden, "you are not a member of this chat")
		return nil, "", false
	}
	return chat, userID, true
}
