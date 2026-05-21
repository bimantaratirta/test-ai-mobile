//go:build integration

package db

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInvites_CreateAndRedeem(t *testing.T) {
	d := NewTestDB(t)
	ctx := context.Background()
	q := Invites{DB: d}

	require.NoError(t, q.Create(ctx, "ABC123"))

	inv, err := q.Get(ctx, "ABC123")
	require.NoError(t, err)
	assert.Equal(t, "ABC123", inv.Code)
	assert.False(t, inv.IsUsed())

	userID := uuid.New()
	// Stub user row (auth.users in real Supabase)
	_, err = d.Pool.Exec(ctx, `insert into auth.users (id) values ($1)`, userID)
	require.NoError(t, err)

	require.NoError(t, q.Redeem(ctx, "ABC123", userID))

	inv, err = q.Get(ctx, "ABC123")
	require.NoError(t, err)
	assert.True(t, inv.IsUsed())
	assert.Equal(t, userID, *inv.UsedBy)
}

func TestInvites_RedeemAlreadyUsed_Fails(t *testing.T) {
	d := NewTestDB(t)
	ctx := context.Background()
	q := Invites{DB: d}

	require.NoError(t, q.Create(ctx, "XYZ"))

	u1, u2 := uuid.New(), uuid.New()
	_, err := d.Pool.Exec(ctx, `insert into auth.users (id) values ($1), ($2)`, u1, u2)
	require.NoError(t, err)

	require.NoError(t, q.Redeem(ctx, "XYZ", u1))
	err = q.Redeem(ctx, "XYZ", u2)
	assert.ErrorIs(t, err, ErrInviteAlreadyUsed)
}

func TestInvites_RedeemUnknown_Fails(t *testing.T) {
	d := NewTestDB(t)
	ctx := context.Background()
	q := Invites{DB: d}

	uid := uuid.New()
	_, err := d.Pool.Exec(ctx, `insert into auth.users (id) values ($1)`, uid)
	require.NoError(t, err)

	err = q.Redeem(ctx, "NOPE", uid)
	assert.ErrorIs(t, err, ErrInviteNotFound)
}
