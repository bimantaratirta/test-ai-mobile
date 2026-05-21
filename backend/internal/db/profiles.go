package db

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Profile struct {
	UserID         uuid.UUID
	DisplayName    *string
	InviteCode     *string
	GeneratesToday int
	GeneratesWeek  int
	ResetAt        time.Time
	CreatedAt      time.Time
}

type Profiles struct{ DB *DB }

func (q Profiles) Upsert(ctx context.Context, userID uuid.UUID, displayName, inviteCode string) error {
	var dn *string
	if displayName != "" {
		dn = &displayName
	}
	var ic *string
	if inviteCode != "" {
		ic = &inviteCode
	}
	_, err := q.DB.Pool.Exec(ctx,
		`insert into profiles (user_id, display_name, invite_code)
		   values ($1, $2, $3)
		 on conflict (user_id) do update
		   set display_name = excluded.display_name,
		       invite_code = coalesce(profiles.invite_code, excluded.invite_code)`,
		userID, dn, ic)
	return err
}

func (q Profiles) Get(ctx context.Context, userID uuid.UUID) (Profile, error) {
	var p Profile
	err := q.DB.Pool.QueryRow(ctx,
		`select user_id, display_name, invite_code, generates_today,
		        generates_week, reset_at, created_at
		   from profiles where user_id = $1`,
		userID).Scan(&p.UserID, &p.DisplayName, &p.InviteCode,
		&p.GeneratesToday, &p.GeneratesWeek, &p.ResetAt, &p.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Profile{}, ErrProfileNotFound
	}
	return p, err
}

func (q Profiles) IncrementCounters(ctx context.Context, userID uuid.UUID) error {
	_, err := q.DB.Pool.Exec(ctx,
		`update profiles
		    set generates_today = generates_today + 1,
		        generates_week  = generates_week  + 1
		  where user_id = $1`, userID)
	return err
}

func (q Profiles) DecrementCounters(ctx context.Context, userID uuid.UUID) error {
	_, err := q.DB.Pool.Exec(ctx,
		`update profiles
		    set generates_today = greatest(generates_today - 1, 0),
		        generates_week  = greatest(generates_week  - 1, 0)
		  where user_id = $1`, userID)
	return err
}

func (q Profiles) ResetDailyCounters(ctx context.Context) error {
	_, err := q.DB.Pool.Exec(ctx,
		`update profiles
		    set generates_today = 0,
		        reset_at = now()`)
	return err
}

func (q Profiles) ResetWeeklyCounters(ctx context.Context) error {
	_, err := q.DB.Pool.Exec(ctx,
		`update profiles set generates_week = 0`)
	return err
}

var ErrProfileNotFound = errors.New("profile not found")
