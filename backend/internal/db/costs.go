package db

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

type Costs struct{ DB *DB }

// AddToday upserts the row for today (UTC) and atomically adds usd to total
// and increments generate_count by 1.
func (q Costs) AddToday(ctx context.Context, usd float64) error {
	_, err := q.DB.Pool.Exec(ctx,
		`insert into daily_costs (date, total_cost_usd, generate_count)
		   values (current_date, $1, 1)
		 on conflict (date) do update
		   set total_cost_usd = daily_costs.total_cost_usd + excluded.total_cost_usd,
		       generate_count = daily_costs.generate_count + 1`, usd)
	return err
}

// Get returns (total cost USD, generate count) for the given date. Returns
// (0, 0, nil) if no row exists.
func (q Costs) Get(ctx context.Context, date time.Time) (float64, int, error) {
	var cost float64
	var count int
	err := q.DB.Pool.QueryRow(ctx,
		`select total_cost_usd, generate_count from daily_costs where date = $1`,
		date).Scan(&cost, &count)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, 0, nil
	}
	return cost, count, err
}
