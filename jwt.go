package main

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID string `json:"user_id"`
	jwt.RegisteredClaims
}

type JWTManager struct {
	secret []byte
	ttl    time.Duration
}

func NewJWTManager(secret string, ttl time.Duration) *JWTManager {
	return &JWTManager{
		secret: []byte(secret),
		ttl:    ttl,
	}
}

func (m *JWTManager) Generate(userID string) (string, error) {
	claims := Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(m.ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)

}

func (m *JWTManager) Verify(tokenStr string) (string, error) {

	claims := &Claims{}

	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {

		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}

		return m.secret, nil
	})

	if err != nil || !token.Valid {
		return "", errors.New("token invalid or expired")
	}

	return claims.UserID, nil
}
