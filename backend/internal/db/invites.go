package db

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	ErrInviteNotFound    = errors.New("invite not found")
	ErrInviteAlreadyUsed = errors.New("invite already used")
)

type Invite struct {
	Code      string
	UsedBy    *uuid.UUID
	CreatedAt time.Time
	UsedAt    *time.Time
}

func (i Invite) IsUsed() bool { return i.UsedBy != nil }

type Invites struct{ DB *DB }

func (q Invites) Create(ctx context.Context, code string) error {
	_, err := q.DB.Pool.Exec(ctx,
		`insert into invites (code) values ($1)`, code)
	return err
}

func (q Invites) Get(ctx context.Context, code string) (Invite, error) {
	var inv Invite
	err := q.DB.Pool.QueryRow(ctx,
		`select code, used_by, created_at, used_at from invites where code = $1`,
		code).Scan(&inv.Code, &inv.UsedBy, &inv.CreatedAt, &inv.UsedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Invite{}, ErrInviteNotFound
	}
	return inv, err
}

// Redeem atomically marks the invite used by userID. Returns ErrInviteAlreadyUsed
// if it's been redeemed, ErrInviteNotFound if it doesn't exist.
func (q Invites) Redeem(ctx context.Context, code string, userID uuid.UUID) error {
	tag, err := q.DB.Pool.Exec(ctx,
		`update invites
		   set used_by = $1, used_at = now()
		 where code = $2 and used_by is null`,
		userID, code)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		// either not found, or already used
		if _, err := q.Get(ctx, code); errors.Is(err, ErrInviteNotFound) {
			return ErrInviteNotFound
		}
		return ErrInviteAlreadyUsed
	}
	return nil
}
