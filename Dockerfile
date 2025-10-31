FROM golang:1.25.3-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

# Копируем весь исходный код
COPY . .

# Собираем приложение
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /app/main ./cmd/main/main.go
FROM alpine:latest

WORKDIR /app

# Копируем миграции, чтобы приложение могло их найти
COPY --from=builder /app/internal/database/migrations ./internal/database/migrations

# Копируем статику для веб-сервера
COPY --from=builder /app/web ./web

# Копируем скомпилированное приложение
COPY --from=builder /app/main .

# Открываем порт, который слушает твой сервер
EXPOSE 8081

# Запускаем приложение
CMD ["/app/main"]