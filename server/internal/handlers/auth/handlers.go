package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/coderianx/Skillora/server/internal/config"
	"github.com/coderianx/Skillora/server/internal/models"
	"github.com/coderianx/Skillora/server/internal/store"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"
)

const (
	tokenTTL        = 30 * time.Minute
	refreshTokenTTL = 7 * 24 * time.Hour
)

func RegisterHandler(
	w http.ResponseWriter,
	r *http.Request,
) {
	w.Header().Set("Content-Type", "application/json")

	var req models.RegisterReq

	err := json.NewDecoder(r.Body).Decode(&req)

	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any{
			"error": "invalid_request",
		})
		return
	}

	if req.Username == "" {
		w.WriteHeader(http.StatusBadRequest)

		json.NewEncoder(w).Encode(map[string]any{
			"error": "username_is_required",
		})

		return
	}

	if req.Password == "" {
		w.WriteHeader(http.StatusBadRequest)

		json.NewEncoder(w).Encode(map[string]any{
			"error": "password_is_required",
		})
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword(
		[]byte(req.Password),
		12,
	)

	if err != nil {
		w.WriteHeader(500)

		json.NewEncoder(w).Encode(map[string]any{
			"error": "bcrypt_generate_failed",
		})
		return
	}

	var userID int64

	err = store.DB.QueryRow(
		r.Context(),
		`
		INSERT INTO users (username, password)
		VALUES ($1,$2)
		RETURNING id;
		`,
		req.Username,
		string(hashedPassword),
	).Scan(&userID)

	if err != nil {
		var pgErr *pgconn.PgError

		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			w.WriteHeader(http.StatusConflict)

			json.NewEncoder(w).Encode(map[string]any{
				"error": "username_already_exists",
			})

			return
		}

		w.WriteHeader(http.StatusInternalServerError)

		json.NewEncoder(w).Encode(map[string]any{
			"error": "user_create_failed",
		})

		return
	}

	w.WriteHeader(http.StatusCreated)

	json.NewEncoder(w).Encode(map[string]any{
		"id":       userID,
		"username": req.Username,
	})
}

func LoginHandler(
	w http.ResponseWriter,
	r *http.Request,
) {
	w.Header().Set("Content-Type", "application/json")

	var req models.LoginReq

	err := json.NewDecoder(r.Body).Decode(&req)

	if err != nil {
		w.WriteHeader(http.StatusBadRequest)

		json.NewEncoder(w).Encode(map[string]any{
			"error": "invalid_request",
		})

		return
	}

	if req.Username == "" {
		w.WriteHeader(http.StatusBadRequest)

		json.NewEncoder(w).Encode(map[string]any{
			"error": "username_is_required",
		})

		return
	}

	if req.Password == "" {
		w.WriteHeader(http.StatusBadRequest)

		json.NewEncoder(w).Encode(map[string]any{
			"error": "password_is_required",
		})

		return
	}

	var userID int64
	var hashedPassword string

	err = store.DB.QueryRow(
		r.Context(),
		`
		SELECT id, password
		FROM users
		WHERE username = $1;
		`,
		req.Username,
	).Scan(&userID, &hashedPassword)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			w.WriteHeader(http.StatusUnauthorized)

			json.NewEncoder(w).Encode(map[string]any{
				"error": "invalid_credentials",
			})

			return
		}

		w.WriteHeader(http.StatusInternalServerError)

		json.NewEncoder(w).Encode(map[string]any{
			"error": "login_failed",
		})

		return
	}

	err = bcrypt.CompareHashAndPassword(
		[]byte(hashedPassword),
		[]byte(req.Password),
	)

	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)

		json.NewEncoder(w).Encode(map[string]any{
			"error": "invalid_credentials",
		})

		return
	}

	accessToken, err := createToken(userID, req.Username)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)

		json.NewEncoder(w).Encode(map[string]any{
			"error": "token_generate_failed",
		})

		return
	}

	refreshToken, err := createRefreshToken(r.Context(), userID)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)

		json.NewEncoder(w).Encode(map[string]any{
			"error": "refresh_token_create_failed",
		})

		return
	}

	w.WriteHeader(http.StatusOK)

	json.NewEncoder(w).Encode(map[string]any{
		"id":           userID,
		"accessToken":  accessToken,
		"refreshToken": refreshToken,
	})
}

func createToken(
	userID int64,
	username string,
) (string, error) {
	now := time.Now()

	claims := models.Claims{
		UserID:   userID,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatInt(userID, 10),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(tokenTTL)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString([]byte(config.JWT_SECRET))
}

func createRefreshToken(
	ctx context.Context,
	userID int64,
) (string, error) {
	raw := make([]byte, 32)

	if _, err := rand.Read(raw); err != nil {
		return "", err
	}

	token := base64.RawURLEncoding.EncodeToString(raw)

	_, err := store.DB.Exec(
		ctx,
		`
		INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3);
		`,
		userID,
		hashToken(token),
		time.Now().Add(refreshTokenTTL),
	)

	if err != nil {
		return "", err
	}

	return token, nil
}

func hashToken(token string) string {
	tokenHash := sha256.Sum256([]byte(token))

	return hex.EncodeToString(tokenHash[:])
}

func RefreshHandler(
	w http.ResponseWriter,
	r *http.Request,
) {
	w.Header().Set("Content-Type", "application/json")

	var req models.RefreshReq

	err := json.NewDecoder(r.Body).Decode(&req)

	if err != nil {
		w.WriteHeader(http.StatusBadRequest)

		json.NewEncoder(w).Encode(map[string]any{
			"error": "invalid_request",
		})

		return
	}

	if req.RefreshToken == "" {
		w.WriteHeader(http.StatusBadRequest)

		json.NewEncoder(w).Encode(map[string]any{
			"error": "refresh_token_is_required",
		})

		return
	}

	var userID int64
	var username string

	// Eski token'ı tek sorguda tüketiyoruz; böylece aynı refresh token
	// eşzamanlı isteklerde ikinci kez kullanılamaz.
	err = store.DB.QueryRow(
		r.Context(),
		`
		DELETE FROM refresh_tokens rt
		USING users u
		WHERE rt.user_id = u.id
		AND rt.token_hash = $1
		AND rt.expires_at > NOW()
		RETURNING u.id, u.username;
		`,
		hashToken(req.RefreshToken),
	).Scan(&userID, &username)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			w.WriteHeader(http.StatusUnauthorized)

			json.NewEncoder(w).Encode(map[string]any{
				"error": "invalid_refresh_token",
			})

			return
		}

		w.WriteHeader(http.StatusInternalServerError)

		json.NewEncoder(w).Encode(map[string]any{
			"error": "refresh_failed",
		})

		return
	}

	accessToken, err := createToken(userID, username)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)

		json.NewEncoder(w).Encode(map[string]any{
			"error": "token_generate_failed",
		})

		return
	}

	refreshToken, err := createRefreshToken(r.Context(), userID)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)

		json.NewEncoder(w).Encode(map[string]any{
			"error": "refresh_token_create_failed",
		})

		return
	}

	w.WriteHeader(http.StatusOK)

	json.NewEncoder(w).Encode(map[string]any{
		"id":           userID,
		"accessToken":  accessToken,
		"refreshToken": refreshToken,
	})
}
