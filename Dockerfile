# Этап 1: Сборка всех бинарников
FROM golang:1.23.3 AS builder

WORKDIR /app

# Сначала только go.mod и go.sum — слой с зависимостями кэшируется отдельно
COPY go.mod go.sum ./
RUN go mod download

# Копируем исходный код
COPY . .

# Собираем монолит, мигратор и все gRPC-микросервисы в один образ
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/main           ./cmd/app/main.go        && \
    CGO_ENABLED=0 GOOS=linux go build -o /out/migrate        ./cmd/migrations/main.go && \
    CGO_ENABLED=0 GOOS=linux go build -o /out/auth-service   ./cmd/auth/main.go       && \
    CGO_ENABLED=0 GOOS=linux go build -o /out/user-service   ./cmd/user/main.go       && \
    CGO_ENABLED=0 GOOS=linux go build -o /out/review-service ./cmd/review/main.go

# Этап 2: Финальный образ
FROM alpine:3.18

WORKDIR /app

# Копируем все собранные бинарники — какой запускать, решает command в compose
COPY --from=builder /out/ ./

# Копируем папку с миграциями
COPY --from=builder /app/db/migrations ./db/migrations

# 8081 — HTTP основного приложения, 50051/50052/50054 — gRPC микросервисов
EXPOSE 8081 50051 50052 50054

CMD ["./main"]
