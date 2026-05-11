package main

import (
	"fmt"
	"log"
	"net/http"
	"time"
)

func main() {
	userStorage := NewUserStorage()
	jwtManager := NewJWTManager("i will change it for prom", time.Hour*24)
	authMW := AuthMiddleware(jwtManager)

	http.HandleFunc("POST /register", handleRegister(userStorage, jwtManager))
	http.HandleFunc("POST /login", handleLogin(userStorage, jwtManager))
	http.Handle("GET /me", authMW(http.HandlerFunc(handleMe(userStorage))))

	fmt.Println("Server started on http://localhost:8080")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal(err)
	}
}
