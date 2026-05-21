package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/bimantara/ai-image/backend/internal/auth"
	"github.com/bimantara/ai-image/backend/internal/db"
)

type RedeemInviteRequest struct {
	Code string `json:"code"`
}

func RedeemInvite(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid, err := auth.UserIDFrom(r.Context())
		if err != nil {
			writeJSONErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		var req RedeemInviteRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Code == "" {
			writeJSONErr(w, http.StatusBadRequest, "invalid request")
			return
		}

		invites := db.Invites{DB: database}
		if err := invites.Redeem(r.Context(), req.Code, uid); err != nil {
			switch {
			case errors.Is(err, db.ErrInviteNotFound):
				writeJSONErr(w, http.StatusBadRequest, "invite code not found")
			case errors.Is(err, db.ErrInviteAlreadyUsed):
				writeJSONErr(w, http.StatusBadRequest, "invite code already used")
			default:
				writeJSONErr(w, http.StatusInternalServerError, "internal error")
			}
			return
		}
		// Initialize profile if needed
		profiles := db.Profiles{DB: database}
		_ = profiles.Upsert(r.Context(), uid, "", req.Code)

		writeJSON(w, http.StatusOK, map[string]bool{"redeemed": true})
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeJSONErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
