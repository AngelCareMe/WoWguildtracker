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
COPY --from=builder /app/wow-guild-tracker .
COPY --from=builder /app/templates ./templates/
EXPOSE 8080
CMD ["./wow-guild-tracker"]