package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/sapfirmoscow/chat-server-go/internal/auth"
	"github.com/sapfirmoscow/chat-server-go/internal/storage"
)

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
