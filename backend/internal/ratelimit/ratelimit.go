package ratelimit

import (
	"context"
	"errors"
	"time"

	"github.com/bimantara/ai-image/backend/internal/db"
	"github.com/google/uuid"
)

var (
	ErrDailyLimit       = errors.New("daily limit exceeded")
	ErrWeeklyLimit      = errors.New("weekly limit exceeded")
	ErrConcurrencyLimit = errors.New("too many in-flight jobs")
	ErrGlobalCostCap    = errors.New("global cost cap reached")
)

type Config struct {
	DB            *db.DB
	PerDay        int
	PerWeek       int
	GlobalCostCap float64
	MaxConcurrent int
}

type Gate struct{ cfg Config }

func New(cfg Config) *Gate { return &Gate{cfg: cfg} }

// Check evaluates all rate-limit and cost-cap rules for the user.
// Returns nil if the request should proceed.
func (g *Gate) Check(ctx context.Context, userID uuid.UUID) error {
	// 1. Global cost cap
	today := time.Now().UTC().Truncate(24 * time.Hour)
	totalCost, _, err := db.Costs{DB: g.cfg.DB}.Get(ctx, today)
	if err != nil {
		return err
	}
	if totalCost >= g.cfg.GlobalCostCap {
		return ErrGlobalCostCap
	}

	// 2. Concurrency
	inFlight, err := db.Jobs{DB: g.cfg.DB}.CountInFlight(ctx, userID)
	if err != nil {
		return err
	}
	if inFlight >= g.cfg.MaxConcurrent {
		return ErrConcurrencyLimit
	}

	// 3. Daily + weekly counters
	prof, err := db.Profiles{DB: g.cfg.DB}.Get(ctx, userID)
	if err != nil {
		return err
	}
	if prof.GeneratesToday >= g.cfg.PerDay {
		return ErrDailyLimit
	}
	if prof.GeneratesWeek >= g.cfg.PerWeek {
		return ErrWeeklyLimit
	}
	return nil
}
