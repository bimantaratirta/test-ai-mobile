package generation

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/bimantara/ai-image/backend/internal/db"
	"github.com/bimantara/ai-image/backend/internal/providers"
	"github.com/bimantara/ai-image/backend/internal/storage"
	"github.com/google/uuid"
)

type WorkerDeps struct {
	DB           *db.DB
	Registry     *providers.Registry
	S3           *storage.S3
	Concurrency  int
	PollInterval time.Duration
	JobTimeout   time.Duration
}

type Worker struct{ d WorkerDeps }

func NewWorker(d WorkerDeps) *Worker {
	if d.Concurrency <= 0 {
		d.Concurrency = 4
	}
	if d.PollInterval <= 0 {
		d.PollInterval = 2 * time.Second
	}
	if d.JobTimeout <= 0 {
		d.JobTimeout = 90 * time.Second
	}
	return &Worker{d: d}
}

func (w *Worker) Run(ctx context.Context) {
	sem := make(chan struct{}, w.d.Concurrency)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		j, err := db.Jobs{DB: w.d.DB}.ClaimNextQueued(ctx, "")
		if errors.Is(err, db.ErrJobNotFound) {
			select {
			case <-ctx.Done():
				return
			case <-time.After(w.d.PollInterval):
				continue
			}
		}
		if err != nil {
			slog.Error("worker: claim failed", "err", err)
			time.Sleep(w.d.PollInterval)
			continue
		}

		sem <- struct{}{}
		go func(job db.Job) {
			defer func() { <-sem }()
			w.process(ctx, job)
		}(j)
	}
}

func (w *Worker) process(parent context.Context, job db.Job) {
	ctx, cancel := context.WithTimeout(parent, w.d.JobTimeout)
	defer cancel()

	jobs := db.Jobs{DB: w.d.DB}

	provider, err := w.d.Registry.For(job.Mode)
	if err != nil {
		_ = jobs.MarkFailed(ctx, job.ID, "no provider")
		return
	}

	// Update the provider field to the actual provider name (may differ from "").
	if err := jobs.MarkProcessing(ctx, job.ID, provider.Name()); err != nil {
		slog.Error("worker: mark processing", "err", err)
		return
	}

	start := time.Now()
	result, err := provider.Generate(ctx, providers.Input{
		InputImageURL: job.InputImageURL,
		Prompt:        job.Prompt,
		StylePreset:   strDeref(job.StylePreset),
	})
	if err != nil {
		w.fail(ctx, job, err)
		return
	}

	key := fmt.Sprintf("outputs/%s.%s", uuid.New(), extFor(result.OutputContentType))
	publicURL, err := w.d.S3.Upload(ctx, key, result.OutputContentType, result.OutputImageBytes)
	if err != nil {
		w.fail(ctx, job, fmt.Errorf("storage_error: %w", err))
		return
	}

	gen := int(time.Since(start).Milliseconds())
	if err := jobs.MarkCompleted(ctx, job.ID, publicURL, result.ProviderRequestID, gen); err != nil {
		slog.Error("worker: mark completed", "err", err)
		return
	}

	if result.EstimatedCostUSD > 0 {
		costs := db.Costs{DB: w.d.DB}
		if err := costs.AddToday(ctx, result.EstimatedCostUSD); err != nil {
			slog.Error("worker: cost add", "err", err)
		}
	}
}

func (w *Worker) fail(ctx context.Context, job db.Job, err error) {
	msg := err.Error()
	if errors.Is(err, providers.ErrTimeout) {
		msg = "timeout"
	}
	if errors.Is(err, providers.ErrContentModerated) {
		msg = "content_moderated: " + msg
	}
	jobs := db.Jobs{DB: w.d.DB}
	if mErr := jobs.MarkFailed(ctx, job.ID, msg); mErr != nil {
		slog.Error("worker: mark failed", "err", mErr)
	}
	// Refund the counter — failed generates don't count against quota
	profiles := db.Profiles{DB: w.d.DB}
	if dErr := profiles.DecrementCounters(ctx, job.UserID); dErr != nil {
		slog.Error("worker: decrement", "err", dErr)
	}
}

func strDeref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func extFor(ct string) string {
	switch ct {
	case "image/png":
		return "png"
	case "image/jpeg":
		return "jpg"
	case "image/webp":
		return "webp"
	default:
		return "bin"
	}
}
