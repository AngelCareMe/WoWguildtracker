# Этап сборки
FROM golang:1.24 AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o wow-guild-tracker ./cmd/server

# Финальный образ
FROM alpine:latest
WORKDIR /root/

# Копируем бинарник
COPY --from=builder /app/wow-guild-tracker .

# Копируем шаблоны и статические файлы
COPY --from=builder /app/templates ./templates/
COPY --from=builder /app/static ./static/

# Указываем порт
EXPOSE 8080

# Запускаем приложение
CMD ["./wow-guild-tracker"]