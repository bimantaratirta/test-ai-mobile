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
	apphttp "github.com/bimantara/ai-image/backend/internal/http"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	cfg, err := config.Load()
	if err != nil {
		slog.Error("config", "err", err); os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	database, err := db.New(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("db", "err", err); os.Exit(1)
	}
	defer database.Close()

	router := apphttp.NewRouter(apphttp.Deps{
		JWTVerifier: auth.NewVerifier(cfg.SupabaseJWTSecret),
	})

	srv := &stdhttp.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 100 * time.Second,
		IdleTimeout:  120 * time.Second,
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
