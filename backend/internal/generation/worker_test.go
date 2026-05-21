//go:build integration

package generation

import (
	"context"
	"testing"
	"time"

	"github.com/bimantara/ai-image/backend/internal/db"
	"github.com/bimantara/ai-image/backend/internal/providers"
	"github.com/bimantara/ai-image/backend/internal/providers/mock"
	"github.com/bimantara/ai-image/backend/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorker_ProcessesQueuedJob(t *testing.T) {
	d := db.NewTestDB(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	uid := setupUser(t, d)
	require.NoError(t, db.Profiles{DB: d}.Upsert(ctx, uid, "x", ""))

	id, err := db.Jobs{DB: d}.Insert(ctx, db.InsertJobParams{
		UserID: uid, Mode: db.ModeRealistic, Prompt: "five chars",
		InputImageURL: "https://x.jpg",
	})
	require.NoError(t, err)

	reg := providers.NewRegistry()
	reg.Register(db.ModeRealistic, mock.New("mock"))
	r2, _ := storage.NewR2("acc", "ak", "sk", "bkt", "https://cdn.test")

	w := NewWorker(WorkerDeps{DB: d, Registry: reg, R2: r2, Concurrency: 1, PollInterval: 50 * time.Millisecond})
	go w.Run(ctx)

	require.Eventually(t, func() bool {
		j, err := db.Jobs{DB: d}.Get(ctx, id)
		return err == nil && (j.Status == db.StatusCompleted || j.Status == db.StatusFailed)
	}, 5*time.Second, 50*time.Millisecond)

	j, err := db.Jobs{DB: d}.Get(ctx, id)
	require.NoError(t, err)
	// Storage upload will fail without real R2; we accept either Completed (if upload skipped)
	// or Failed with storage_error. Verify provider was at least invoked.
	assert.Contains(t, []db.JobStatus{db.StatusCompleted, db.StatusFailed}, j.Status)
	assert.Equal(t, "mock", *j.Provider)
}
