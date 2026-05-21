package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/bimantara/ai-image/backend/internal/auth"
	"github.com/bimantara/ai-image/backend/internal/storage"
	"github.com/google/uuid"
)

type PresignRequest struct {
	ContentType string `json:"content_type"`
}

type PresignResponse struct {
	UploadURL string `json:"upload_url"`
	PublicURL string `json:"public_url"`
	Key       string `json:"key"`
}

func PresignUpload(r2 *storage.S3) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid, err := auth.UserIDFrom(r.Context())
		if err != nil {
			writeJSONErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		var req PresignRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.ContentType == "" {
			req.ContentType = "image/jpeg"
		}
		if !isAllowedImageType(req.ContentType) {
			writeJSONErr(w, http.StatusBadRequest, "unsupported content type")
			return
		}

		key := "uploads/" + uid.String() + "/" + uuid.New().String() + extFor(req.ContentType)
		upload, public, err := r2.PresignPut(r.Context(), key, req.ContentType, 5*time.Minute)
		if err != nil {
			writeJSONErr(w, http.StatusInternalServerError, "presign failed")
			return
		}
		writeJSON(w, http.StatusOK, PresignResponse{UploadURL: upload, PublicURL: public, Key: key})
	}
}

func isAllowedImageType(ct string) bool {
	switch ct {
	case "image/jpeg", "image/png", "image/webp", "image/heic":
		return true
	}
	return false
}

func extFor(ct string) string {
	switch ct {
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "image/heic":
		return ".heic"
	default:
		return ".jpg"
	}
}
