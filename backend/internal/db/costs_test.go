//go:build integration

package db

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCosts_AddAndGetToday(t *testing.T) {
	d := NewTestDB(t)
	ctx := context.Background()
	q := Costs{DB: d}

	require.NoError(t, q.AddToday(ctx, 0.04))
	require.NoError(t, q.AddToday(ctx, 0.06))

	today := time.Now().UTC().Truncate(24 * time.Hour)
	cost, count, err := q.Get(ctx, today)
	require.NoError(t, err)
	assert.InEpsilon(t, 0.10, cost, 0.0001)
	assert.Equal(t, 2, count)
}

func TestCosts_GetTodayWhenEmpty(t *testing.T) {
	d := NewTestDB(t)
	ctx := context.Background()
	q := Costs{DB: d}

	cost, count, err := q.Get(ctx, time.Now().UTC().Truncate(24*time.Hour))
	require.NoError(t, err)
	assert.Equal(t, 0.0, cost)
	assert.Equal(t, 0, count)
}
