//go:build integration

package db

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDB_PingsAndAppliesSchema(t *testing.T) {
	d := NewTestDB(t)

	var count int
	err := d.Pool.QueryRow(context.Background(),
		`select count(*) from information_schema.tables
		 where table_name in ('jobs', 'profiles', 'invites', 'daily_costs')`).
		Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 4, count)
}
