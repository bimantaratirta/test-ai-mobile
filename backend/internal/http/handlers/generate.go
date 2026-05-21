package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/bimantara/ai-image/backend/internal/auth"
	"github.com/bimantara/ai-image/backend/internal/db"
	"github.com/bimantara/ai-image/backend/internal/generation"
	"github.com/bimantara/ai-image/backend/internal/ratelimit"
)

type GenerateRequest struct {
	InputImageURL string `json:"input_image_url"`
	Mode          string `json:"mode"`
	Prompt        string `json:"prompt"`
	StylePreset   string `json:"style_preset"`
}

type GenerateResponse struct {
	JobID            string `json:"job_id"`
	Status           string `json:"status"`
	EstimatedSeconds int    `json:"estimated_seconds"`
}

func Generate(svc *generation.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid, err := auth.UserIDFrom(r.Context())
		if err != nil {
			writeJSONErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		var req GenerateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSONErr(w, http.StatusBadRequest, "invalid json")
			return
		}
		id, err := svc.Submit(r.Context(), generation.SubmitInput{
			UserID:        uid,
			Mode:          db.JobMode(req.Mode),
			Prompt:        req.Prompt,
			StylePreset:   req.StylePreset,
			InputImageURL: req.InputImageURL,
		})
		if err != nil {
			switch {
			case errors.Is(err, generation.ErrInvalidInput):
				writeJSONErr(w, http.StatusBadRequest, "invalid input")
			case errors.Is(err, ratelimit.ErrDailyLimit), errors.Is(err, ratelimit.ErrWeeklyLimit):
				w.Header().Set("Retry-After", "3600")
				writeJSONErr(w, http.StatusTooManyRequests, "rate limit reached")
			case errors.Is(err, ratelimit.ErrConcurrencyLimit):
				writeJSONErr(w, http.StatusTooManyRequests, "too many in-flight jobs")
			case errors.Is(err, ratelimit.ErrGlobalCostCap):
				w.Header().Set("Retry-After", "3600")
				writeJSONErr(w, http.StatusServiceUnavailable, "service paused for the day")
			default:
				writeJSONErr(w, http.StatusInternalServerError, "internal error")
			}
			return
		}
		writeJSON(w, http.StatusOK, GenerateResponse{
			JobID: id.String(), Status: "queued", EstimatedSeconds: 25,
		})
	}
}
