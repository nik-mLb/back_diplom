package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/go-park-mail-ru/2025_1_ChillGuys/internal/models/domains"
	gen "github.com/go-park-mail-ru/2025_1_ChillGuys/internal/transport/generated/auth"
	"github.com/go-park-mail-ru/2025_1_ChillGuys/internal/transport/jwt"
	"github.com/go-park-mail-ru/2025_1_ChillGuys/internal/transport/utils/response"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/metadata"
)

// OptionalJWTMiddleware извлекает userID из токена, если он присутствует и валиден,
// но НЕ отклоняет запрос при отсутствии/невалидности токена — просто пропускает
// дальше без userID в контексте. Нужен для эндпоинтов, работающих и для гостя, и
// для залогиненного пользователя (например, рекомендации на главной странице).
func OptionalJWTMiddleware(authClient gen.AuthServiceClient, tokenator *jwt.Tokenator, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		cookieValue, err := r.Cookie(string(domains.TokenCookieName))
		if err != nil || cookieValue.Value == "" {
			next.ServeHTTP(w, r)
			return
		}
		tokenString := cookieValue.Value

		checkResp, err := authClient.CheckToken(ctx, &gen.CheckTokenReq{Token: tokenString})
		if err != nil || !checkResp.Valid {
			next.ServeHTTP(w, r)
			return
		}

		claims, err := tokenator.ParseJWT(tokenString)
		if err != nil || claims.ExpiresAt < time.Now().Unix() {
			next.ServeHTTP(w, r)
			return
		}

		ctx = context.WithValue(ctx, domains.UserIDKey{}, claims.UserID)
		ctx = context.WithValue(ctx, domains.RoleKey{}, claims.Role)
		ctx = metadata.AppendToOutgoingContext(ctx, "user-id", claims.UserID, "role", claims.Role)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// JWTMiddleware проверяет наличие и валидность JWT-токена в куках
func JWTMiddleware(authClient gen.AuthServiceClient, tokenator *jwt.Tokenator, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Получаем request_id из контекста
		reqID := ctx.Value(domains.ReqIDKey{})
		if reqID == nil {
			reqID = "unknown"
		}

		// Логирование с request_id и remote_addr
		requestLogger := logrus.WithFields(logrus.Fields{
			"request_id":  reqID,
			"remote_addr": r.RemoteAddr,
			"path":        r.URL.Path,
		})

		cookieValue, err := r.Cookie(string(domains.TokenCookieName))
		if err != nil {
			requestLogger.Warn("Missing or invalid token cookie")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Получаем токен из куки
		tokenString := cookieValue.Value

		// Если токен пустой, возвращаем ошибку
		if tokenString == "" {
			requestLogger.WithFields(logrus.Fields{
				"method": r.Method,
				"path":   r.URL.Path,
				"ip":     r.RemoteAddr,
			}).Warn("Empty token")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Вызываем gRPC метод CheckToken для проверки токена
		checkResp, err := authClient.CheckToken(ctx, &gen.CheckTokenReq{
			Token: tokenString,
		})
		if err != nil {
			requestLogger.WithError(err).Error("Failed to check token via gRPC")
			response.SendJSONError(ctx, w, http.StatusUnauthorized, "Invalid token")
			return
		}

		if !checkResp.Valid {
			requestLogger.Warn("Token is not valid")
			response.SendJSONError(ctx, w, http.StatusUnauthorized, "Invalid token")
			return
		}

		// Дополнительно парсим токен для получения claims (если нужно)
		claims, err := tokenator.ParseJWT(tokenString)
		if err != nil {
			requestLogger.WithError(err).Error("Failed to parse token after gRPC validation")
			response.SendJSONError(ctx, w, http.StatusUnauthorized, "Invalid token")
			return
		}

		// Проверяем, не истёк ли токен
		if claims.ExpiresAt < time.Now().Unix() {
			requestLogger.Warn("Token expired")
			response.SendJSONError(ctx, w, http.StatusUnauthorized, "Token expired")
			return
		}

		// Передаем userID и role в контекст
		ctx = context.WithValue(ctx, domains.UserIDKey{}, claims.UserID)
		ctx = context.WithValue(ctx, domains.RoleKey{}, claims.Role)

		// Добавляем user-id в метаданные для gRPC
		ctx = metadata.AppendToOutgoingContext(ctx, 
			"user-id", claims.UserID,
			"role", claims.Role,  // <- Добавьте эту строку
		)
		
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
