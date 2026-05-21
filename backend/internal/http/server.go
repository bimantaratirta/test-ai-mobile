package http

import (
	stdhttp "net/http"
	"time"

	"github.com/bimantara/ai-image/backend/internal/auth"
	"github.com/bimantara/ai-image/backend/internal/http/handlers"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type Deps struct {
	JWTVerifier *auth.Verifier
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
		// authenticated routes wired in later tasks
	})

	return r
}
