//go:build integration

package generation

import (
	"context"
	"testing"

	"github.com/bimantara/ai-image/backend/internal/db"
	"github.com/bimantara/ai-image/backend/internal/providers"
	"github.com/bimantara/ai-image/backend/internal/providers/mock"
	"github.com/bimantara/ai-image/backend/internal/ratelimit"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubmit_HappyPath(t *testing.T) {
	d := db.NewTestDB(t)
	ctx := context.Background()
	uid := setupUser(t, d)
	require.NoError(t, db.Profiles{DB: d}.Upsert(ctx, uid, "x", ""))

	reg := providers.NewRegistry()
	reg.Register(db.ModeRealistic, mock.New("mock-realistic"))
	gate := ratelimit.New(ratelimit.Config{
		DB: d, PerDay: 10, PerWeek: 30, GlobalCostCap: 25, MaxConcurrent: 3,
	})
	svc := NewService(Deps{DB: d, Registry: reg, Gate: gate})

	id, err := svc.Submit(ctx, SubmitInput{
		UserID: uid, Mode: db.ModeRealistic, Prompt: "test", InputImageURL: "https://x.jpg",
	})
	require.NoError(t, err)

	j, err := db.Jobs{DB: d}.Get(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, db.StatusQueued, j.Status)

	// counter incremented immediately at submit (decremented on failure)
	p, err := db.Profiles{DB: d}.Get(ctx, uid)
	require.NoError(t, err)
	assert.Equal(t, 1, p.GeneratesToday)
}

func TestSubmit_RateLimited(t *testing.T) {
	d := db.NewTestDB(t)
	ctx := context.Background()
	uid := setupUser(t, d)
	require.NoError(t, db.Profiles{DB: d}.Upsert(ctx, uid, "x", ""))
	require.NoError(t, db.Profiles{DB: d}.IncrementCounters(ctx, uid))

	reg := providers.NewRegistry()
	reg.Register(db.ModeRealistic, mock.New("mock"))
	gate := ratelimit.New(ratelimit.Config{
		DB: d, PerDay: 1, PerWeek: 30, GlobalCostCap: 25, MaxConcurrent: 3,
	})
	svc := NewService(Deps{DB: d, Registry: reg, Gate: gate})

	_, err := svc.Submit(ctx, SubmitInput{
		UserID: uid, Mode: db.ModeRealistic, Prompt: "p", InputImageURL: "https://x",
	})
	assert.ErrorIs(t, err, ratelimit.ErrDailyLimit)
}

func TestSubmit_InvalidPrompt(t *testing.T) {
	d := db.NewTestDB(t)
	ctx := context.Background()
	uid := setupUser(t, d)
	require.NoError(t, db.Profiles{DB: d}.Upsert(ctx, uid, "x", ""))

	reg := providers.NewRegistry()
	reg.Register(db.ModeRealistic, mock.New("mock"))
	gate := ratelimit.New(ratelimit.Config{DB: d, PerDay: 10, PerWeek: 30, GlobalCostCap: 25, MaxConcurrent: 3})
	svc := NewService(Deps{DB: d, Registry: reg, Gate: gate})

	_, err := svc.Submit(ctx, SubmitInput{
		UserID: uid, Mode: db.ModeRealistic, Prompt: "abc", InputImageURL: "https://x",
	})
	assert.ErrorIs(t, err, ErrInvalidInput)
}

func setupUser(t *testing.T, d *db.DB) uuid.UUID {
	t.Helper()
	uid := uuid.New()
	_, err := d.Pool.Exec(context.Background(), `insert into auth.users (id) values ($1)`, uid)
	require.NoError(t, err)
	return uid
}
