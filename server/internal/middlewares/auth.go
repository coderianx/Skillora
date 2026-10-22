package middlewares

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/coderianx/Skillora/server/internal/config"
	"github.com/coderianx/Skillora/server/internal/models"
	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const claimsContextKey contextKey = "claims"

// Auth, Authorization: Bearer <token> başlığındaki JWT'yi doğrular ve
// claim'leri request context'ine ekler.
// r.With(middlewares.Auth).Get(...) şeklinde kullanılır.
func Auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		parts := strings.Fields(r.Header.Get("Authorization"))

		if len(parts) == 0 {
			writeError(w, http.StatusUnauthorized, "missing_authorization_header")
			return
		}

		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			writeError(w, http.StatusUnauthorized, "invalid_authorization_header")
			return
		}

		claims := &models.Claims{}

		token, err := jwt.ParseWithClaims(
			parts[1],
			claims,
			func(*jwt.Token) (any, error) {
				return []byte(config.JWT_SECRET), nil
			},
			jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		)

		if err != nil || !token.Valid {
			if errors.Is(err, jwt.ErrTokenExpired) {
				writeError(w, http.StatusUnauthorized, "token_expired")
				return
			}

			writeError(w, http.StatusUnauthorized, "invalid_token")
			return
		}

		ctx := context.WithValue(r.Context(), claimsContextKey, claims)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// ClaimsFromContext, Auth middleware'inin context'e eklediği claim'leri döndürür.
func ClaimsFromContext(ctx context.Context) (*models.Claims, bool) {
	claims, ok := ctx.Value(claimsContextKey).(*models.Claims)

	return claims, ok
}

func writeError(
	w http.ResponseWriter,
	status int,
	code string,
) {
	w.Header().Set("Content-Type", "application/json")

	if status == http.StatusUnauthorized {
		w.Header().Set("WWW-Authenticate", "Bearer")
	}

	w.WriteHeader(status)

	json.NewEncoder(w).Encode(map[string]any{
		"error": code,
	})
}
