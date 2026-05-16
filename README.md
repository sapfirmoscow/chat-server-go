# chat-server-go

Бэкенд мессенджера на Go — REST API + WebSocket для обмена сообщениями в реальном времени.

## Стек

- **Go** (стандартная библиотека `net/http`, роутинг через паттерны Go 1.22+)
- **JWT** — `golang-jwt/jwt/v5`
- **WebSocket** — `coder/websocket`
- **UUID** — `google/uuid`
- **Хранилище** — in-memory (синхронизация через `sync.RWMutex`)

## Быстрый старт

```bash
git clone https://github.com/sapfirmoscow/chat-server-go
cd chat-server-go
go run ./cmd/server
# Сервер запустится на http://localhost:8080
```

## API

Полная спецификация — [`api-spec.yaml`](./api-spec.yaml) (OpenAPI 3.0).

### Аутентификация

Все защищённые эндпоинты требуют заголовок:
```
Authorization: Bearer <JWT_TOKEN>
```
Токен выдаётся при регистрации или логине, действителен **24 часа**.

### Эндпоинты

| Метод | Путь | Описание |
|-------|------|----------|
| `POST` | `/register` | Регистрация пользователя |
| `POST` | `/login` | Вход, получение токена |
| `GET` | `/me` | Профиль текущего пользователя |
| `POST` | `/chats` | Создать DM-чат с пользователем |
| `GET` | `/chats` | Список своих чатов |
| `POST` | `/chats/{id}/messages` | Отправить сообщение |
| `GET` | `/chats/{id}/messages` | История сообщений (с пагинацией) |
| `GET` | `/ws` | WebSocket-соединение |

### WebSocket

Подключение: `ws://localhost:8080/ws`

Токен передаётся через субпротокол:
```
Sec-WebSocket-Protocol: access_token, <JWT_TOKEN>
```

**События сервер → клиент:**

```json
{
  "type": "message.new",
  "data": {
    "id": "...",
    "chat_id": "...",
    "sender_id": "...",
    "text": "Привет!",
    "created_at": "2026-05-16T10:10:00Z"
  }
}
```

Один пользователь может иметь несколько одновременных соединений (телефон + десктоп).

## Структура проекта

```
cmd/server/          — точка входа
internal/
  auth/              — JWT-менеджер и middleware
  handlers/          — HTTP-обработчики (auth, user, chat, message, ws)
  models/            — модели данных (User, Chat, Message)
  storage/           — in-memory хранилища
  ws/                — WebSocket Hub и Client
api-spec.yaml        — OpenAPI-спецификация
```

## Заметки

- Данные хранятся **в памяти** и сбрасываются при перезапуске сервера. Далее будет в БД
- JWT-секрет захардкожен — перед деплоем вынести в переменную окружения.
