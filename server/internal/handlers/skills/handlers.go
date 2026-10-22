package skills

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/coderianx/Skillora/server/internal/middlewares"
	"github.com/coderianx/Skillora/server/internal/models"
	"github.com/coderianx/Skillora/server/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func CreateNewSkillHandler(
	w http.ResponseWriter,
	r *http.Request,
) {
	w.Header().Set("Content-Type", "application/json")

	claims, ok := middlewares.ClaimsFromContext(r.Context())

	if !ok {
		w.WriteHeader(http.StatusUnauthorized)

		json.NewEncoder(w).Encode(map[string]any{
			"error": "unauthorized",
		})
		return
	}

	var req models.CreateNewSkill

	err := json.NewDecoder(r.Body).Decode(&req)

	if err != nil {
		w.WriteHeader(http.StatusBadRequest)

		json.NewEncoder(w).Encode(map[string]any{
			"error": "invalid_request",
		})
		return
	}

	req.Title = strings.TrimSpace(req.Title)
	req.Description = strings.TrimSpace(req.Description)
	req.Content = strings.TrimSpace(req.Content)

	if req.Title == "" {
		w.WriteHeader(http.StatusBadRequest)

		json.NewEncoder(w).Encode(map[string]any{
			"error": "title_is_required",
		})
		return
	}

	if req.Description == "" {
		w.WriteHeader(http.StatusBadRequest)

		json.NewEncoder(w).Encode(map[string]any{
			"error": "description_is_required",
		})
		return
	}

	if req.Content == "" {
		w.WriteHeader(http.StatusBadRequest)

		json.NewEncoder(w).Encode(map[string]any{
			"error": "content_is_required",
		})
		return
	}

	if len(req.Title) > 50 {
		w.WriteHeader(http.StatusBadRequest)

		json.NewEncoder(w).Encode(map[string]any{
			"error": "title_is_too_long",
		})
		return
	}

	if len(req.Description) > 250 {
		w.WriteHeader(http.StatusBadRequest)

		json.NewEncoder(w).Encode(map[string]any{
			"error": "description_is_too_long",
		})
		return
	}

	skillID := claims.Username + "/" + req.Title

	_, err = store.DB.Exec(
		r.Context(),
		`
		INSERT INTO skills (id, user_id, title, description)
		VALUES ($1, $2, $3, $4);
		`,
		skillID,
		claims.UserID,
		req.Title,
		req.Description,
	)

	if err != nil {
		var pgErr *pgconn.PgError

		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			w.WriteHeader(http.StatusConflict)

			json.NewEncoder(w).Encode(map[string]any{
				"error": "skill_already_exists",
			})
			return
		}

		w.WriteHeader(http.StatusInternalServerError)

		json.NewEncoder(w).Encode(map[string]any{
			"error": "skill_create_failed",
		})
		return
	}

	w.WriteHeader(http.StatusCreated)

	json.NewEncoder(w).Encode(map[string]any{
		"id": skillID,
	})
}

func DeleteSkillHandler(
	w http.ResponseWriter,
	r *http.Request,
) {
	w.Header().Set("Content-Type", "application/json")

	claims, ok := middlewares.ClaimsFromContext(r.Context())

	if !ok {
		w.WriteHeader(http.StatusUnauthorized)

		json.NewEncoder(w).Encode(map[string]any{
			"error": "unauthorized",
		})
		return
	}

	title := strings.TrimSpace(chi.URLParam(r, "id"))

	if title == "" {
		w.WriteHeader(http.StatusBadRequest)

		json.NewEncoder(w).Encode(map[string]any{
			"error": "skill_id_is_required",
		})
		return
	}

	skillID := claims.Username + "/" + title

	tag, err := store.DB.Exec(
		r.Context(),
		`
		DELETE FROM skills
		WHERE id = $1 AND user_id = $2;
		`,
		skillID,
		claims.UserID,
	)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)

		json.NewEncoder(w).Encode(map[string]any{
			"error": "skill_delete_failed",
		})
		return
	}

	if tag.RowsAffected() == 0 {
		w.WriteHeader(http.StatusNotFound)

		json.NewEncoder(w).Encode(map[string]any{
			"error": "skill_not_found",
		})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
