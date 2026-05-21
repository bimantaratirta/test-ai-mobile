//go:build integration

package db

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJobs_InsertAndGet(t *testing.T) {
	d := NewTestDB(t)
	ctx := context.Background()
	q := Jobs{DB: d}

	uid := setupUser(t, d)
	id, err := q.Insert(ctx, InsertJobParams{
		UserID:        uid,
		Mode:          ModeRealistic,
		Prompt:        "test",
		StylePreset:   "scandinavian",
		InputImageURL: "https://cdn/x.jpg",
	})
	require.NoError(t, err)

	job, err := q.Get(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, ModeRealistic, job.Mode)
	assert.Equal(t, StatusQueued, job.Status)
}

func TestJobs_MarkProcessingThenCompleted(t *testing.T) {
	d := NewTestDB(t)
	ctx := context.Background()
	q := Jobs{DB: d}
	uid := setupUser(t, d)

	id, err := q.Insert(ctx, InsertJobParams{
		UserID: uid, Mode: ModeInspirational, Prompt: "p",
		InputImageURL: "https://cdn/i.jpg",
	})
	require.NoError(t, err)

	require.NoError(t, q.MarkProcessing(ctx, id, "flux"))
	require.NoError(t, q.MarkCompleted(ctx, id, "https://cdn/o.jpg", "req-1", 12345))

	j, err := q.Get(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, StatusCompleted, j.Status)
	assert.Equal(t, "https://cdn/o.jpg", *j.OutputImageURL)
	assert.Equal(t, 12345, *j.GenerationMs)
}

func TestJobs_MarkFailed(t *testing.T) {
	d := NewTestDB(t)
	ctx := context.Background()
	q := Jobs{DB: d}
	uid := setupUser(t, d)

	id, err := q.Insert(ctx, InsertJobParams{
		UserID: uid, Mode: ModeRealistic, Prompt: "p",
		InputImageURL: "https://cdn/i.jpg",
	})
	require.NoError(t, err)
	require.NoError(t, q.MarkFailed(ctx, id, "timeout"))

	j, err := q.Get(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, StatusFailed, j.Status)
	assert.Equal(t, "timeout", *j.ErrorMessage)
}

func TestJobs_CountInFlight(t *testing.T) {
	d := NewTestDB(t)
	ctx := context.Background()
	q := Jobs{DB: d}
	uid := setupUser(t, d)

	for i := 0; i < 3; i++ {
		_, err := q.Insert(ctx, InsertJobParams{
			UserID: uid, Mode: ModeRealistic, Prompt: "p",
			InputImageURL: "https://x",
		})
		require.NoError(t, err)
	}
	n, err := q.CountInFlight(ctx, uid)
	require.NoError(t, err)
	assert.Equal(t, 3, n)
}

func TestJobs_SweepStale(t *testing.T) {
	d := NewTestDB(t)
	ctx := context.Background()
	q := Jobs{DB: d}
	uid := setupUser(t, d)

	id, err := q.Insert(ctx, InsertJobParams{
		UserID: uid, Mode: ModeRealistic, Prompt: "p",
		InputImageURL: "https://x",
	})
	require.NoError(t, err)
	require.NoError(t, q.MarkProcessing(ctx, id, "gemini"))

	// Backdate to 10 minutes ago
	_, err = d.Pool.Exec(ctx,
		`update jobs set created_at = now() - interval '10 minutes' where id = $1`, id)
	require.NoError(t, err)

	n, err := q.SweepStale(ctx, 5*time.Minute)
	require.NoError(t, err)
	assert.Equal(t, 1, n)

	j, err := q.Get(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, StatusFailed, j.Status)
	assert.Equal(t, "timeout", *j.ErrorMessage)
}

func TestJobs_List(t *testing.T) {
	d := NewTestDB(t)
	ctx := context.Background()
	q := Jobs{DB: d}
	uid := setupUser(t, d)

	for i := 0; i < 5; i++ {
		_, err := q.Insert(ctx, InsertJobParams{
			UserID: uid, Mode: ModeRealistic, Prompt: "p",
			InputImageURL: "https://x",
		})
		require.NoError(t, err)
	}
	list, err := q.List(ctx, uid, 3, time.Now().Add(time.Hour))
	require.NoError(t, err)
	assert.Len(t, list, 3)
}

func setupUser(t *testing.T, d *DB) uuid.UUID {
	t.Helper()
	uid := uuid.New()
	_, err := d.Pool.Exec(context.Background(),
		`insert into auth.users (id) values ($1)`, uid)
	require.NoError(t, err)
	return uid
}
