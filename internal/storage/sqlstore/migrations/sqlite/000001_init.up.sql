CREATE TABLE users (
    id            TEXT PRIMARY KEY,
    username      TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at    DATETIME NOT NULL
);

-- direct_key задан только у личных чатов: UNIQUE не даёт создать
-- второй чат для той же пары пользователей.
CREATE TABLE chats (
    id         TEXT PRIMARY KEY,
    direct_key TEXT UNIQUE,
    created_at DATETIME NOT NULL
);

CREATE TABLE chat_members (
    chat_id  TEXT    NOT NULL REFERENCES chats (id) ON DELETE CASCADE,
    user_id  TEXT    NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    position INTEGER NOT NULL,
    PRIMARY KEY (chat_id, user_id)
);

CREATE INDEX chat_members_user_id_idx ON chat_members (user_id);

-- seq задаёт стабильный порядок сообщений внутри чата.
CREATE TABLE messages (
    seq        INTEGER PRIMARY KEY AUTOINCREMENT,
    id         TEXT NOT NULL UNIQUE,
    chat_id    TEXT NOT NULL REFERENCES chats (id) ON DELETE CASCADE,
    sender_id  TEXT NOT NULL REFERENCES users (id),
    text       TEXT NOT NULL,
    created_at DATETIME NOT NULL
);

CREATE INDEX messages_chat_id_seq_idx ON messages (chat_id, seq);
