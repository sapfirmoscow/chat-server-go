# syntax=docker/dockerfile:1

# --- сборка ---
FROM golang:1.26-alpine AS build
WORKDIR /src

# Зависимости отдельным слоем: кэш не сбрасывается при изменении кода.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# CGO не нужен: SQLite-драйвер modernc.org/sqlite написан на чистом Go.
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server \
    && mkdir -p /out/data

# --- рантайм: только бинарник, без shell и пакетного менеджера ---
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/server /app/server
# Каталог для файла SQLite (STORAGE_DRIVER=sqlite, DATABASE_URL=/data/chat.db).
COPY --from=build --chown=nonroot:nonroot /out/data /data
VOLUME /data

ENV PORT=8080
EXPOSE 8080
USER nonroot:nonroot

ENTRYPOINT ["/app/server"]
