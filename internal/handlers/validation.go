package handlers

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// usernameRe — допустимый username (уже приведённый к нижнему регистру):
// только латиница, цифры, "_" и ".". Буквы других алфавитов запрещены, чтобы
// нельзя было завести ник, визуально неотличимый от чужого (латинская "a"
// и кириллическая "а" выглядят одинаково).
var usernameRe = regexp.MustCompile(`^[a-z0-9_.]{3,32}$`)

const (
	minUsernameRunes = 3
	maxUsernameRunes = 32
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

// validateUsername проверяет username при регистрации; ожидает уже нормализованное значение.
func validateUsername(username string) string {
	if !usernameRe.MatchString(username) {
		return fmt.Sprintf("username must be %d-%d characters: latin letters, digits, '_' or '.'",
			minUsernameRunes, maxUsernameRunes)
	}
	return ""
}

// normalizeUsername приводит username к виду, в котором он хранится.
func normalizeUsername(username string) string {
	return strings.ToLower(strings.TrimSpace(username))
}
