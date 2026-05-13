package handlers

import (
	"encoding/json"
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

func HandleCreateChat(chatStorage *storage.Chats, userStorage *storage.Users) http.HandlerFunc {
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
		_, exist := userStorage.GetByID(req.MemberID)
		if !exist {
			WriteError(w, http.StatusNotFound, "member not found")
			return
		}

		//check exist chat for both
		if chat, exist := chatStorage.FindDirectChat(req.MemberID, currentUserID); exist {
			//just return already exists chat
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			response := BuildChatResponse(chat, userStorage)

			if err := json.NewEncoder(w).Encode(response); err != nil {
				log.Printf("encode error: %v", err)
			}
			return
		}

		//create new
		newChat := models.Chat{
			ID:        uuid.NewString(),
			MemberIDs: []string{currentUserID, req.MemberID},
			CreatedAt: time.Now().UTC(),
		}

		if err := chatStorage.Add(&newChat); err != nil {
			log.Printf("chat add error: %v", err)
			WriteError(w, http.StatusInternalServerError, "failed to create chat")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		response := BuildChatResponse(&newChat, userStorage)

		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("encode error: %v", err)
		}

	}
}

func HandleGetMyChats(chatStorage *storage.Chats, userStorage *storage.Users) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		currentUserID, ok := auth.GetUserID(r.Context())

		if !ok {
			WriteError(w, http.StatusInternalServerError, "Cant found userID in context")
			return
		}

		chats := chatStorage.GetUserChats(currentUserID)

		responses := make([]ChatResponse, 0, len(chats))

		for _, chat := range chats {
			responses = append(responses, BuildChatResponse(chat, userStorage))
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
