# chat-server-go

Бэкенд мессенджера на Go — REST API + WebSocket для обмена сообщениями в реальном времени.

## Стек

- **Go** (стандартная библиотека `net/http`, роутинг через паттерны Go 1.22+)
- **JWT** — `golang-jwt/jwt/v5`
- **WebSocket** — `coder/websocket`
- **UUID** — `google/uuid`
- **Хранилище** — PostgreSQL (`jackc/pgx/v5`), SQLite (`modernc.org/sqlite`, без CGO) или in-memory
- **Миграции** — `golang-migrate/migrate/v4`, применяются автоматически при старте

## Быстрый старт

```bash
git clone https://github.com/sapfirmoscow/chat-server-go
cd chat-server-go
export JWT_SECRET=$(openssl rand -hex 32)
go run ./cmd/server
# Сервер запустится на http://localhost:8080, данные хранятся в памяти до перезапуска
```

PowerShell:

```powershell
$b = New-Object byte[] 32; [Security.Cryptography.RandomNumberGenerator]::Create().GetBytes($b)
$env:JWT_SECRET = -join ($b | ForEach-Object { $_.ToString('x2') })
go run ./cmd/server
```

### Конфигурация

Все настройки — переменные окружения:

| Переменная | Значения | По умолчанию |
|---|---|---|
| `JWT_SECRET` | секрет подписи токенов, не короче 32 байт | **обязательна**, без неё сервер не стартует |
| `PORT` | порт HTTP-сервера | `8080` |
| `STORAGE_DRIVER` | `memory`, `sqlite`, `postgres` | `memory` |
| `DATABASE_URL` | путь к файлу SQLite или URL PostgreSQL | `chat.db` для SQLite, для PostgreSQL обязателен |

Если сменить `JWT_SECRET`, все ранее выданные токены станут недействительными.

### Хранилище

PostgreSQL локально через Docker:

```bash
docker compose up -d
STORAGE_DRIVER=postgres \
DATABASE_URL="postgres://chat:chat@localhost:5432/chat?sslmode=disable" \
go run ./cmd/server
```

Миграции лежат в `internal/storage/sqlstore/migrations/{postgres,sqlite}`, встраиваются в бинарник
и применяются при каждом запуске (уже применённые пропускаются). Новая миграция — пара файлов
`00000N_name.up.sql` / `00000N_name.down.sql` для каждого диалекта.

`memory` (по умолчанию) хранит данные только до перезапуска, это удобно для быстрых экспериментов.
Чтобы данные сохранялись в файле без установки чего-либо, задайте `STORAGE_DRIVER=sqlite`.

### Docker

Сервер вместе с PostgreSQL:

```bash
echo "JWT_SECRET=$(openssl rand -hex 32)" > .env   # .env в .gitignore
docker compose --profile app up -d --build
# Сервер: http://localhost:8080
```

Без `--profile app` поднимается только PostgreSQL, это удобно для `go run` на хосте.

Только образ (in-memory или SQLite в volume):

```bash
docker build -t chat-server .
docker run -p 8080:8080 -e JWT_SECRET=... chat-server
docker run -p 8080:8080 -e JWT_SECRET=...   -e STORAGE_DRIVER=sqlite -e DATABASE_URL=/data/chat.db -v chat-data:/data chat-server
```

Образ собран на distroless: внутри только статический бинарник, процесс работает не от root.
По `docker stop` (SIGTERM) сервер дожидается текущих запросов и закрывает WebSocket-соединения.

### Тесты

```bash
go test ./...
# Тот же набор тестов на PostgreSQL (таблицы в этой БД будут очищены):
TEST_POSTGRES_URL="postgres://chat:chat@localhost:5432/chat?sslmode=disable" go test ./internal/storage/...
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
| `GET` | `/users/by-username/{username}` | Поиск пользователя по точному username |
| `POST` | `/chats` | Создать DM-чат с пользователем |
| `GET` | `/chats` | Список своих чатов |
| `POST` | `/chats/{id}/messages` | Отправить сообщение |
| `GET` | `/chats/{id}/messages` | История сообщений (с пагинацией) |
| `GET` | `/ws` | WebSocket-соединение |

### Регистрация

```
POST /register {"username": "alice", "password": "secret1"}
→ 201 {"token": "...", "user": {...}}
```

- **username** — 3–32 символа: латинские буквы, цифры, `_` и `.`. Регистр и пробелы по краям
  нормализуются (`" Alice "` → `alice`). Буквы других алфавитов запрещены намеренно, чтобы нельзя
  было зарегистрировать ник, визуально неотличимый от чужого (латинская `a` и кириллическая `а`).
- **password** — от 6 символов и не длиннее 72 байт (ограничение bcrypt).

### Поиск пользователей

```
GET /users/by-username/{username}
→ 200 {"id": "...", "username": "..."}
→ 404 {"error": "user not found"}
```

Нужен для создания DM-чата: `POST /chats` требует `member_id`, а взять его больше неоткуда.

Особенности реализации (осознанные решения):
- **Поиск точный, а не по подстроке.** Точное совпадение возвращает максимум одну запись,
  поэтому выкачать реестр пользователей перебором нельзя. Подстрочный поиск или list-all
  позволили бы одному авторизованному аккаунту собрать всю базу (username + UUID).
- **Отдаются только `id` и `username`** — без `created_at`.
- **Закрыт авторизацией** (`auth.Middleware`), как `/me` и `/chats`.
- **Ограничение частоты: 20 запросов в минуту на пользователя** (`internal/ratelimit`),
  сверх лимита — `429` с заголовком `Retry-After`. Гасит автоматический перебор username.
- Ввод нормализуется (`TrimSpace` + `ToLower`) так же, как при регистрации.


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

При остановке сервера соединения закрываются с кодом `1001` (going away): клиенту нужно переподключиться.

## Структура проекта

```
cmd/server/          — точка входа
internal/
  auth/              — JWT-менеджер и middleware
  handlers/          — HTTP-обработчики (auth, user, chat, message, ws)
  models/            — модели данных (User, Chat, Message)
  ratelimit/         — ограничение частоты запросов (middleware)
  storage/           — интерфейсы хранилищ
    memory/          — in-memory реализация
    sqlstore/        — PostgreSQL / SQLite + миграции
    storagetest/     — общий набор тестов для всех реализаций
  ws/                — WebSocket Hub и Client
api-spec.yaml        — OpenAPI-спецификация
Dockerfile           — образ сервера (distroless)
docker-compose.yml   — PostgreSQL и сервер (профиль app)
```
