package obs

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/getsentry/sentry-go"
	sentryhttp "github.com/getsentry/sentry-go/http"
)

// Init configures Sentry; no-op if dsn is empty.
func Init(dsn, env string) error {
	if dsn == "" {
		slog.Info("sentry disabled (no DSN)")
		return nil
	}
	return sentry.Init(sentry.ClientOptions{
		Dsn:              dsn,
		Environment:      env,
		TracesSampleRate: 0.1,
	})
}

// Middleware wraps an http.Handler with Sentry's panic recovery + request scope.
func Middleware(next http.Handler) http.Handler {
	return sentryhttp.New(sentryhttp.Options{Repanic: true}).Handle(next)
}

// Flush should be deferred from main before shutdown.
func Flush() { sentry.Flush(2 * time.Second) }
