//go:build integration

package db

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProfiles_UpsertAndGet(t *testing.T) {
	d := NewTestDB(t)
	ctx := context.Background()
	q := Profiles{DB: d}

	uid := uuid.New()
	_, err := d.Pool.Exec(ctx, `insert into auth.users (id) values ($1)`, uid)
	require.NoError(t, err)

	require.NoError(t, q.Upsert(ctx, uid, "BetaBob", "INVITE1"))
	p, err := q.Get(ctx, uid)
	require.NoError(t, err)
	assert.Equal(t, "BetaBob", *p.DisplayName)
	assert.Equal(t, 0, p.GeneratesToday)
}

func TestProfiles_IncrementCounters(t *testing.T) {
	d := NewTestDB(t)
	ctx := context.Background()
	q := Profiles{DB: d}

	uid := uuid.New()
	_, err := d.Pool.Exec(ctx, `insert into auth.users (id) values ($1)`, uid)
	require.NoError(t, err)
	require.NoError(t, q.Upsert(ctx, uid, "x", ""))

	require.NoError(t, q.IncrementCounters(ctx, uid))
	require.NoError(t, q.IncrementCounters(ctx, uid))

	p, err := q.Get(ctx, uid)
	require.NoError(t, err)
	assert.Equal(t, 2, p.GeneratesToday)
	assert.Equal(t, 2, p.GeneratesWeek)
}

func TestProfiles_ResetDaily(t *testing.T) {
	d := NewTestDB(t)
	ctx := context.Background()
	q := Profiles{DB: d}

	uid := uuid.New()
	_, err := d.Pool.Exec(ctx, `insert into auth.users (id) values ($1)`, uid)
	require.NoError(t, err)
	require.NoError(t, q.Upsert(ctx, uid, "x", ""))
	require.NoError(t, q.IncrementCounters(ctx, uid))

	require.NoError(t, q.ResetDailyCounters(ctx))
	p, err := q.Get(ctx, uid)
	require.NoError(t, err)
	assert.Equal(t, 0, p.GeneratesToday)
	assert.Equal(t, 1, p.GeneratesWeek, "weekly counter survives daily reset")
}
