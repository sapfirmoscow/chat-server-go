package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type RegisterRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type CreateChatRequest struct {
	MemberID string `json:"member_id"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type AuthResponse struct {
	Token string     `json:"token"`
	User  PublicUser `json:"user"`
}

type ChatResponse struct {
	ID        string       `json:"id"`
	Members   []PublicUser `json:"members"`
	CreatedAt time.Time    `json:"created_at"`
}

type ChatsResponse struct {
	Chats []ChatResponse `json:"chats"`
}

func handleMe(userStorage *UserStorage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		userID, ok := GetUserID(r.Context())

		if !ok {
			writeError(w, http.StatusInternalServerError, "Cant found userID in context")
			return
		}

		user, ok := userStorage.GetByID(userID)

		if !ok {
			writeError(w, http.StatusNotFound, "User not found")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		if err := json.NewEncoder(w).Encode(user.ToPublic()); err != nil {
			log.Printf("encode error: %v", err)

		}
	}
}

func handleRegister(userStorage *UserStorage, jwtManager *JWTManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req RegisterRequest

		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()

		err := decoder.Decode(&req)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid json")
			return
		}

		req.Username = strings.TrimSpace(strings.ToLower(req.Username))

		if len(req.Username) < 3 || len(req.Username) > 32 {
			writeError(w, http.StatusBadRequest, "username must be 3-32 characters")
			return
		}
		if len(req.Password) < 6 {
			writeError(w, http.StatusBadRequest, "password must be at least 6 characters")
			return
		}

		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		newUser := User{
			ID:           uuid.NewString(),
			Username:     req.Username,
			PasswordHash: string(hash),
			CreatedAt:    time.Now().UTC(),
		}

		err = userStorage.Add(&newUser)

		if errors.Is(err, ErrUsernameTaken) {
			writeError(w, http.StatusConflict, "username already taken")

			return
		} else if err != nil {
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		token, err := jwtManager.Generate(newUser.ID)

		if err != nil {
			log.Printf("generation token error: %v", err)
			writeError(w, http.StatusInternalServerError, "cant generate token")
			return
		}

		response := AuthResponse{
			Token: token,
			User:  newUser.ToPublic(),
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)

		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("encode error: %v", err)

		}
	}
}

func handleLogin(userStorage *UserStorage, jwtManager *JWTManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req LoginRequest

		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()

		err := decoder.Decode(&req)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid json")
			return
		}

		req.Username = strings.TrimSpace(strings.ToLower(req.Username))

		user, found := userStorage.GetByUsername(req.Username)
		if !found {
			writeError(w, http.StatusUnauthorized, "invalid credentials")
			return
		}

		if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
			writeError(w, http.StatusUnauthorized, "invalid credentials")
			return
		}

		token, err := jwtManager.Generate(user.ID)

		if err != nil {
			log.Printf("generation token error: %v", err)
			writeError(w, http.StatusInternalServerError, "cant generate token")
			return
		}

		response := AuthResponse{
			Token: token,
			User:  user.ToPublic(),
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("encode error: %v", err)
		}
	}
}

func handleCreateChat(chatStorage *ChatStorage, userStorage *UserStorage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		currentUserID, ok := GetUserID(r.Context())

		if !ok {
			writeError(w, http.StatusInternalServerError, "Cant found userID in context")
			return
		}
		var req CreateChatRequest

		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()

		err := decoder.Decode(&req)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid json")
			return
		}

		//check id is empty
		if req.MemberID == "" {
			writeError(w, http.StatusBadRequest, "member id is empty")
			return
		}

		//check is chat with yourself
		if req.MemberID == currentUserID {
			writeError(w, http.StatusBadRequest, "can't create chat with yourself")
			return
		}

		//check that member exist
		_, exist := userStorage.GetByID(req.MemberID)
		if !exist {
			writeError(w, http.StatusNotFound, "member not found")
			return
		}

		//check exist chat for both
		if chat, exist := chatStorage.FindDirectChat(req.MemberID, currentUserID); exist {
			//just return already exists chat
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			response := buildChatResponse(chat, userStorage)

			if err := json.NewEncoder(w).Encode(response); err != nil {
				log.Printf("encode error: %v", err)
			}
			return
		}

		//create new
		newChat := Chat{
			ID:        uuid.NewString(),
			MemberIDs: []string{currentUserID, req.MemberID},
			CreatedAt: time.Now().UTC(),
		}

		if err := chatStorage.Add(&newChat); err != nil {
			log.Printf("chat add error: %v", err)
			writeError(w, http.StatusInternalServerError, "failed to create chat")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		response := buildChatResponse(&newChat, userStorage)

		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("encode error: %v", err)
		}

	}
}

func handleGetMyChats(chatStorage *ChatStorage, userStorage *UserStorage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		currentUserID, ok := GetUserID(r.Context())

		if !ok {
			writeError(w, http.StatusInternalServerError, "Cant found userID in context")
			return
		}

		chats := chatStorage.GetUserChats(currentUserID)

		responses := make([]ChatResponse, 0, len(chats))

		for _, chat := range chats {
			responses = append(responses, buildChatResponse(chat, userStorage))
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

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(ErrorResponse{Error: message}); err != nil {
		log.Printf("encode error: %v", err)
	}
}

func GetUserID(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(userIDKey).(string)
	return userID, ok
}

func buildChatResponse(chat *Chat, userStorage *UserStorage) ChatResponse {
	members := make([]PublicUser, 0, len(chat.MemberIDs))

	for _, id := range chat.MemberIDs {
		if user, ok := userStorage.GetByID(id); ok {
			members = append(members, user.ToPublic())
		}
	}

	return ChatResponse{
		ID:        chat.ID,
		Members:   members,
		CreatedAt: chat.CreatedAt,
	}
}
