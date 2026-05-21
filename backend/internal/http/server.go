package http

import (
	stdhttp "net/http"
	"time"

	"github.com/bimantara/ai-image/backend/internal/auth"
	"github.com/bimantara/ai-image/backend/internal/db"
	"github.com/bimantara/ai-image/backend/internal/generation"
	"github.com/bimantara/ai-image/backend/internal/http/handlers"
	"github.com/bimantara/ai-image/backend/internal/storage"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type Deps struct {
	JWTVerifier *auth.Verifier
	DB          *db.DB
	R2          *storage.R2
	Generation  *generation.Service
}

func NewRouter(d Deps) stdhttp.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	r.Get("/health", handlers.Health)

	r.Group(func(r chi.Router) {
		r.Use(auth.Required(d.JWTVerifier))
		r.Post("/redeem-invite", handlers.RedeemInvite(d.DB))
		r.Post("/uploads/presign", handlers.PresignUpload(d.R2))
		r.Post("/generate", handlers.Generate(d.Generation))
		r.Get("/jobs", handlers.ListJobs(d.DB))
		r.Get("/jobs/{id}", handlers.GetJob(d.DB))
	})

	return r
}
