package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/bimantara/ai-image/backend/internal/auth"
	"github.com/bimantara/ai-image/backend/internal/db"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type JobDTO struct {
	ID             string  `json:"id"`
	Mode           string  `json:"mode"`
	Status         string  `json:"status"`
	Prompt         string  `json:"prompt"`
	StylePreset    *string `json:"style_preset"`
	InputImageURL  string  `json:"input_image_url"`
	OutputImageURL *string `json:"output_image_url"`
	ErrorMessage   *string `json:"error_message"`
	GenerationMs   *int    `json:"generation_ms"`
	CreatedAt      string  `json:"created_at"`
	CompletedAt    *string `json:"completed_at"`
}

func toDTO(j db.Job) JobDTO {
	dto := JobDTO{
		ID: j.ID.String(), Mode: string(j.Mode), Status: string(j.Status),
		Prompt: j.Prompt, StylePreset: j.StylePreset, InputImageURL: j.InputImageURL,
		OutputImageURL: j.OutputImageURL, ErrorMessage: j.ErrorMessage,
		GenerationMs: j.GenerationMs, CreatedAt: j.CreatedAt.UTC().Format(time.RFC3339),
	}
	if j.CompletedAt != nil {
		s := j.CompletedAt.UTC().Format(time.RFC3339)
		dto.CompletedAt = &s
	}
	return dto
}

func ListJobs(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid, err := auth.UserIDFrom(r.Context())
		if err != nil {
			writeJSONErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		q := r.URL.Query()
		limit, _ := strconv.Atoi(q.Get("limit"))
		if limit <= 0 || limit > 50 {
			limit = 20
		}
		cursor := time.Now().UTC().Add(time.Hour)
		if c := q.Get("cursor"); c != "" {
			if t, err := time.Parse(time.RFC3339, c); err == nil {
				cursor = t
			}
		}
		jobs := db.Jobs{DB: database}
		list, err := jobs.List(r.Context(), uid, limit, cursor)
		if err != nil {
			writeJSONErr(w, http.StatusInternalServerError, "list failed")
			return
		}
		out := make([]JobDTO, 0, len(list))
		for _, j := range list {
			out = append(out, toDTO(j))
		}
		writeJSON(w, http.StatusOK, map[string]any{"jobs": out})
	}
}

func GetJob(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid, err := auth.UserIDFrom(r.Context())
		if err != nil {
			writeJSONErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		id, err := uuid.Parse(chi.URLParam(r, "id"))
		if err != nil {
			writeJSONErr(w, http.StatusBadRequest, "invalid id")
			return
		}
		jobs := db.Jobs{DB: database}
		j, err := jobs.Get(r.Context(), id)
		if err != nil {
			if errors.Is(err, db.ErrJobNotFound) {
				writeJSONErr(w, http.StatusNotFound, "not found")
				return
			}
			writeJSONErr(w, http.StatusInternalServerError, "internal error")
			return
		}
		if j.UserID != uid {
			writeJSONErr(w, http.StatusNotFound, "not found")
			return
		}
		writeJSON(w, http.StatusOK, toDTO(j))
	}
}
