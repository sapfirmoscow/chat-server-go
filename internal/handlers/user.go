package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/sapfirmoscow/chat-server-go/internal/auth"
	"github.com/sapfirmoscow/chat-server-go/internal/storage"
)

// FindUserResponse — минимальные данные пользователя, достаточные для создания DM-чата.
// Намеренно не содержит created_at: чем меньше отдаём в поиске, тем лучше.
type FindUserResponse struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}

func HandleMe(userStorage *storage.Users) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		userID, ok := auth.GetUserID(r.Context())

		if !ok {
			WriteError(w, http.StatusInternalServerError, "Cant found userID in context")
			return
		}

		user, ok := userStorage.GetByID(userID)

		if !ok {
			WriteError(w, http.StatusNotFound, "User not found")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		if err := json.NewEncoder(w).Encode(user.ToPublic()); err != nil {
			log.Printf("encode error: %v", err)

		}
	}
}

// HandleFindUserByUsername ищет пользователя по точному username.
//
// Поиск намеренно точный, а не по подстроке: точное совпадение возвращает
// максимум одну запись, поэтому выкачать реестр пользователей перебором нельзя.
// Эндпоинт должен быть закрыт auth.Middleware и ограничен по частоте запросов.
func HandleFindUserByUsername(userStorage *storage.Users) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Нормализация как при регистрации — там username приводится к нижнему регистру.
		username := strings.ToLower(strings.TrimSpace(r.PathValue("username")))

		if len(username) < 3 || len(username) > 32 {
			WriteError(w, http.StatusBadRequest, "username must be 3-32 characters")
			return
		}

		user, found := userStorage.GetByUsername(username)
		if !found {
			WriteError(w, http.StatusNotFound, "user not found")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		response := FindUserResponse{
			ID:       user.ID,
			Username: user.Username,
		}

		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("encode error: %v", err)
		}
	}
}
