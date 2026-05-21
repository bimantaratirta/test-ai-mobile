//go:build integration

package db

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// NewTestDB spins up a real Postgres in a container and applies migrations.
// Build tag `integration` keeps it out of fast unit runs.
func NewTestDB(t *testing.T) *DB {
	t.Helper()
	ctx := context.Background()

	container, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("postgres:16-alpine"),
		postgres.WithDatabase("test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = container.Terminate(ctx) })

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	d, err := New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(d.Close)

	// Apply migrations. The 001_initial.sql references auth.users, auth.uid()
	// and the supabase_realtime publication — for tests we substitute stubs.
	migration := loadMigration(t)
	stub := `
		create schema if not exists auth;
		create table if not exists auth.users (id uuid primary key);
		create or replace function auth.uid() returns uuid as $$ select null::uuid $$
		  language sql stable;
		create publication if not exists supabase_realtime;
	`
	_, err = d.Pool.Exec(ctx, stub)
	require.NoError(t, err)

	// Strip the realtime publication line from the migration (already created).
	cleaned := strings.ReplaceAll(migration,
		"alter publication supabase_realtime add table jobs;", "")
	_, err = d.Pool.Exec(ctx, cleaned)
	require.NoError(t, err)

	return d
}

func loadMigration(t *testing.T) string {
	t.Helper()
	// Walk up to find migrations file (test runs from package dir).
	path, err := filepath.Abs("migrations/001_initial.sql")
	require.NoError(t, err)
	b, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(b)
}
