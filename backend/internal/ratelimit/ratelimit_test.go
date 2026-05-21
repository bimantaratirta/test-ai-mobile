//go:build integration

package ratelimit

import (
	"context"
	"testing"

	"github.com/bimantara/ai-image/backend/internal/db"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheck_AllowsWithinLimits(t *testing.T) {
	d := db.NewTestDB(t)
	ctx := context.Background()
	gate := New(Config{
		DB:            d,
		PerDay:        5,
		PerWeek:       10,
		GlobalCostCap: 25.0,
		MaxConcurrent: 3,
	})

	uid := setupUser(t, d)
	require.NoError(t, db.Profiles{DB: d}.Upsert(ctx, uid, "x", ""))

	require.NoError(t, gate.Check(ctx, uid))
}

func TestCheck_DailyLimitExceeded(t *testing.T) {
	d := db.NewTestDB(t)
	ctx := context.Background()
	gate := New(Config{DB: d, PerDay: 2, PerWeek: 10, GlobalCostCap: 25, MaxConcurrent: 3})

	uid := setupUser(t, d)
	require.NoError(t, db.Profiles{DB: d}.Upsert(ctx, uid, "x", ""))
	require.NoError(t, db.Profiles{DB: d}.IncrementCounters(ctx, uid))
	require.NoError(t, db.Profiles{DB: d}.IncrementCounters(ctx, uid))

	err := gate.Check(ctx, uid)
	assert.ErrorIs(t, err, ErrDailyLimit)
}

func TestCheck_ConcurrencyLimitExceeded(t *testing.T) {
	d := db.NewTestDB(t)
	ctx := context.Background()
	gate := New(Config{DB: d, PerDay: 100, PerWeek: 100, GlobalCostCap: 25, MaxConcurrent: 2})

	uid := setupUser(t, d)
	require.NoError(t, db.Profiles{DB: d}.Upsert(ctx, uid, "x", ""))
	jobs := db.Jobs{DB: d}
	for i := 0; i < 2; i++ {
		_, err := jobs.Insert(ctx, db.InsertJobParams{
			UserID: uid, Mode: db.ModeRealistic, Prompt: "p", InputImageURL: "https://x",
		})
		require.NoError(t, err)
	}

	err := gate.Check(ctx, uid)
	assert.ErrorIs(t, err, ErrConcurrencyLimit)
}

func TestCheck_GlobalCostCap(t *testing.T) {
	d := db.NewTestDB(t)
	ctx := context.Background()
	gate := New(Config{DB: d, PerDay: 100, PerWeek: 100, GlobalCostCap: 0.05, MaxConcurrent: 3})

	require.NoError(t, db.Costs{DB: d}.AddToday(ctx, 0.06))

	uid := setupUser(t, d)
	require.NoError(t, db.Profiles{DB: d}.Upsert(ctx, uid, "x", ""))

	err := gate.Check(ctx, uid)
	assert.ErrorIs(t, err, ErrGlobalCostCap)
}

func setupUser(t *testing.T, d *db.DB) uuid.UUID {
	t.Helper()
	uid := uuid.New()
	_, err := d.Pool.Exec(context.Background(), `insert into auth.users (id) values ($1)`, uid)
	require.NoError(t, err)
	return uid
}
