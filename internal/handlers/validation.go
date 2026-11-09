package handlers

import (
	"fmt"
	"unicode/utf8"
)

const (
	minPasswordRunes = 6
	// maxPasswordBytes — предел bcrypt: длиннее 72 байт GenerateFromPassword
	// возвращает ошибку, и раньше клиент получал 500.
	maxPasswordBytes = 72
)

// validatePassword возвращает текст ошибки для клиента или "" если пароль подходит.
// Минимум считается в символах, максимум — в байтах (так считает bcrypt).
func validatePassword(password string) string {
	if utf8.RuneCountInString(password) < minPasswordRunes {
		return fmt.Sprintf("password must be at least %d characters", minPasswordRunes)
	}
	if len(password) > maxPasswordBytes {
		return fmt.Sprintf("password must be at most %d bytes", maxPasswordBytes)
	}
	return ""
}
