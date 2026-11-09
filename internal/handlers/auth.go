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

type AuthResponse struct {
	Token string            `json:"token"`
	User  models.PublicUser `json:"user"`
}

func HandleRegister(userStorage storage.Users, jwtManager *auth.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req RegisterRequest

		if !DecodeJSON(w, r, &req) {
			return
		}

		req.Username = normalizeUsername(req.Username)

		if msg := validateUsername(req.Username); msg != "" {
			WriteError(w, http.StatusBadRequest, msg)
			return
		}
		if msg := validatePassword(req.Password); msg != "" {
			WriteError(w, http.StatusBadRequest, msg)
			return
		}

		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		newUser := models.User{
			ID:           uuid.NewString(),
			Username:     req.Username,
			PasswordHash: string(hash),
			CreatedAt:    time.Now().UTC(),
		}

		err = userStorage.Add(r.Context(), &newUser)

		if errors.Is(err, storage.ErrUsernameTaken) {
			WriteError(w, http.StatusConflict, "username already taken")

			return
		} else if err != nil {
			log.Printf("add user error: %v", err)
			WriteError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		token, err := jwtManager.Generate(newUser.ID)

		if err != nil {
			log.Printf("generation token error: %v", err)
			WriteError(w, http.StatusInternalServerError, "cant generate token")
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

func HandleLogin(userStorage storage.Users, jwtManager *auth.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req LoginRequest

		if !DecodeJSON(w, r, &req) {
			return
		}

		req.Username = normalizeUsername(req.Username)

		user, err := userStorage.GetByUsername(r.Context(), req.Username)
		if errors.Is(err, storage.ErrNotFound) {
			WriteError(w, http.StatusUnauthorized, "invalid credentials")
			return
		} else if err != nil {
			log.Printf("get user error: %v", err)
			WriteError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
			WriteError(w, http.StatusUnauthorized, "invalid credentials")
			return
		}

		token, err := jwtManager.Generate(user.ID)

		if err != nil {
			log.Printf("generation token error: %v", err)
			WriteError(w, http.StatusInternalServerError, "cant generate token")
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
