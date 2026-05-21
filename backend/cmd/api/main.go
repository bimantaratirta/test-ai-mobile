package main

import (
	"context"
	"errors"
	"log/slog"
	stdhttp "net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bimantara/ai-image/backend/internal/auth"
	"github.com/bimantara/ai-image/backend/internal/config"
	"github.com/bimantara/ai-image/backend/internal/db"
	"github.com/bimantara/ai-image/backend/internal/generation"
	apphttp "github.com/bimantara/ai-image/backend/internal/http"
	"github.com/bimantara/ai-image/backend/internal/obs"
	"github.com/bimantara/ai-image/backend/internal/providers"
	"github.com/bimantara/ai-image/backend/internal/providers/flux"
	"github.com/bimantara/ai-image/backend/internal/providers/gemini"
	"github.com/bimantara/ai-image/backend/internal/ratelimit"
	"github.com/bimantara/ai-image/backend/internal/storage"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	cfg, err := config.Load()
	if err != nil {
		slog.Error("config", "err", err); os.Exit(1)
	}

	if err := obs.Init(cfg.SentryDSN, cfg.Env); err != nil {
		slog.Error("sentry init", "err", err)
	}
	defer obs.Flush()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	database, err := db.New(ctx, cfg.DatabaseURL)
	if err != nil { slog.Error("db", "err", err); os.Exit(1) }
	defer database.Close()

	r2, err := storage.NewR2(cfg.R2AccountID, cfg.R2AccessKeyID, cfg.R2SecretAccessKey, cfg.R2Bucket, cfg.R2PublicBaseURL)
	if err != nil { slog.Error("r2", "err", err); os.Exit(1) }

	reg := providers.NewRegistry()
	reg.Register(db.ModeRealistic, gemini.New(gemini.Config{APIKey: cfg.GeminiAPIKey}))
	reg.Register(db.ModeInspirational, flux.New(flux.Config{
		Token: cfg.ReplicateAPIToken, ModelVersion: cfg.ReplicateFluxVersion,
	}))

	gate := ratelimit.New(ratelimit.Config{
		DB: database, PerDay: cfg.RateLimitPerDay, PerWeek: cfg.RateLimitPerWeek,
		GlobalCostCap: cfg.GlobalCostCapUSD, MaxConcurrent: cfg.MaxConcurrentJobsPerUser,
	})
	svc := generation.NewService(generation.Deps{DB: database, Registry: reg, Gate: gate})

	worker := generation.NewWorker(generation.WorkerDeps{
		DB: database, Registry: reg, R2: r2, Concurrency: 4,
	})
	go worker.Run(ctx)

	router := obs.Middleware(apphttp.NewRouter(apphttp.Deps{
		JWTVerifier: auth.NewVerifier(cfg.SupabaseJWTSecret),
		DB: database, R2: r2, Generation: svc,
	}))

	srv := &stdhttp.Server{
		Addr: ":" + cfg.Port, Handler: router,
		ReadTimeout: 10 * time.Second, WriteTimeout: 100 * time.Second,
		IdleTimeout: 120 * time.Second,
	}
	done := make(chan struct{})
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, syscall.SIGINT, syscall.SIGTERM)
		<-sigint
		sctx, scancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer scancel()
		_ = srv.Shutdown(sctx)
		close(done)
	}()

	slog.Info("listening", "port", cfg.Port, "env", cfg.Env)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, stdhttp.ErrServerClosed) {
		slog.Error("server", "err", err); os.Exit(1)
	}
	<-done
}
