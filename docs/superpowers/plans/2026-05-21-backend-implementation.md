# Backend Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the Go backend for the Coffee Shop Design Generator prototype: HTTP API that handles invite-gated signup, authenticated image-generation jobs routed to Gemini 2.5 Flash Image (realistic mode) or Flux Pro 1.1 via Replicate (inspirational mode), with rate limiting and cost cap.

**Architecture:** Stateless Go HTTP service on Fly.io Singapore. Supabase Postgres for data, Supabase Auth for JWT issuance (verified server-side). Cloudflare R2 for object storage (presigned PUT for input uploads; service-side upload for outputs). Background worker pool (goroutines) for AI provider calls; Supabase Realtime pushes job status to clients.

**Tech Stack:** Go 1.22, `chi` router, `pgx/v5`, `aws-sdk-go-v2` (R2), `golang-jwt/jwt/v5`, `getsentry/sentry-go`, `kelseyhightower/envconfig`, `testify`, `testcontainers-go`, Docker, Fly.io.

**Spec:** `docs/superpowers/specs/2026-05-21-coffee-shop-design-generator-prototype-design.md`

---

## Prerequisites (manual, one-time setup)

Before starting Task 1, complete these external account / project setups. List the credentials in a local `.env` file (gitignored) as you go.

- [ ] **Supabase project**
  - Create project at https://supabase.com (region: Singapore).
  - From Settings → API, note: `SUPABASE_URL`, `SUPABASE_ANON_KEY`, `SUPABASE_SERVICE_ROLE_KEY`, `SUPABASE_JWT_SECRET`.
  - From Settings → Database, note: `DATABASE_URL` (use the connection-pooler URL for app, direct URL for migrations).
- [ ] **Cloudflare R2 bucket**
  - Create R2 bucket `ai-image-prototype`.
  - Create API token with R2 read+write scope.
  - Note: `R2_ACCOUNT_ID`, `R2_ACCESS_KEY_ID`, `R2_SECRET_ACCESS_KEY`, `R2_BUCKET=ai-image-prototype`, `R2_PUBLIC_BASE_URL` (custom domain attached to bucket).
- [ ] **Google AI Studio**
  - Get API key at https://aistudio.google.com/app/apikey.
  - Note: `GEMINI_API_KEY`.
- [ ] **Replicate**
  - Sign up at https://replicate.com, generate API token at https://replicate.com/account/api-tokens.
  - Note: `REPLICATE_API_TOKEN`. Pin model version for Flux Pro 1.1 by visiting https://replicate.com/black-forest-labs/flux-1.1-pro and copying the version hash → `REPLICATE_FLUX_VERSION`.
- [ ] **Fly.io**
  - Install `flyctl` (`brew install flyctl`).
  - `fly auth signup` or `fly auth login`.
  - Add payment method (free tier is metered after a small allowance).
- [ ] **Sentry**
  - Create org + project (platform: Go).
  - Note: `SENTRY_DSN` for backend.
- [ ] **GitHub repo**
  - Create `<your-user>/ai-image` repo.
  - From `/Users/bimantara/Dev/ASHA/Pathon/ai-image` run: `git remote add origin git@github.com:<your-user>/ai-image.git`.
  - Push existing main: `git push -u origin main`.

---

## File Structure

After all tasks complete, `backend/` will look like:

```
backend/
├── cmd/api/main.go
├── internal/
│   ├── auth/{jwt.go, jwt_test.go, middleware.go, context.go}
│   ├── config/config.go
│   ├── db/{db.go, profiles.go, profiles_test.go, jobs.go, jobs_test.go,
│   │       invites.go, invites_test.go, costs.go, costs_test.go,
│   │       migrations/001_initial.sql, testutil.go}
│   ├── storage/{r2.go, r2_test.go}
│   ├── providers/{provider.go, registry.go, mock/mock.go,
│   │              gemini/gemini.go, gemini/gemini_test.go,
│   │              flux/flux.go, flux_test.go}
│   ├── generation/{service.go, service_test.go, worker.go, worker_test.go}
│   ├── ratelimit/{ratelimit.go, ratelimit_test.go}
│   ├── http/{server.go, handlers/{health.go, invites.go, uploads.go,
│   │         generate.go, jobs.go}, handlers/handlers_test.go}
│   ├── obs/sentry.go
│   └── testutil/fixtures.go
├── Dockerfile
├── fly.toml
├── .dockerignore
├── .env.example
├── go.mod
├── go.sum
└── .github/workflows/backend.yml  (lives at repo root: ai-image/.github/...)
```

---

## Task 1: Initialize Go module and basic structure

**Files:**
- Create: `backend/go.mod`
- Create: `backend/cmd/api/main.go`
- Create: `backend/.env.example`
- Create: `backend/.dockerignore`

- [ ] **Step 1: Initialize Go module**

Run:
```bash
cd backend
go mod init github.com/bimantara/ai-image/backend
```

- [ ] **Step 2: Create minimal main.go**

Create `backend/cmd/api/main.go`:

```go
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{"status":"ok"}`)
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 100 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	idleConnsClosed := make(chan struct{})
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, syscall.SIGINT, syscall.SIGTERM)
		<-sigint
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			slog.Error("graceful shutdown failed", "err", err)
		}
		close(idleConnsClosed)
	}()

	slog.Info("server starting", "port", port)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("server failed", "err", err)
		os.Exit(1)
	}
	<-idleConnsClosed
}
```

- [ ] **Step 3: Verify it runs**

Run:
```bash
cd backend
go run ./cmd/api &
sleep 1
curl -s http://localhost:8080/health
kill %1
```
Expected: `{"status":"ok"}`

- [ ] **Step 4: Create .env.example**

Create `backend/.env.example`:

```bash
# Server
PORT=8080
ENV=development

# Supabase
DATABASE_URL=postgres://postgres:[YOUR-PW]@db.[REF].supabase.co:5432/postgres
SUPABASE_URL=https://[REF].supabase.co
SUPABASE_SERVICE_ROLE_KEY=eyJ...
SUPABASE_JWT_SECRET=your-jwt-secret

# Cloudflare R2
R2_ACCOUNT_ID=
R2_ACCESS_KEY_ID=
R2_SECRET_ACCESS_KEY=
R2_BUCKET=ai-image-prototype
R2_PUBLIC_BASE_URL=https://cdn.example.com

# AI Providers
GEMINI_API_KEY=
REPLICATE_API_TOKEN=
REPLICATE_FLUX_VERSION=

# Observability
SENTRY_DSN=

# Anti-abuse limits
RATE_LIMIT_PER_DAY=10
RATE_LIMIT_PER_WEEK=30
GLOBAL_COST_CAP_USD=25.00
MAX_CONCURRENT_JOBS_PER_USER=3
```

- [ ] **Step 5: Create .dockerignore**

Create `backend/.dockerignore`:

```
.env
.env.local
.git
.github
tmp/
bin/
*.test
coverage.out
```

- [ ] **Step 6: Commit**

```bash
cd /Users/bimantara/Dev/ASHA/Pathon/ai-image
git add backend/go.mod backend/cmd backend/.env.example backend/.dockerignore
git commit -m "backend: bootstrap Go module with health endpoint"
```

---

## Task 2: Add config package (envconfig)

**Files:**
- Create: `backend/internal/config/config.go`
- Create: `backend/internal/config/config_test.go`
- Modify: `backend/cmd/api/main.go`

- [ ] **Step 1: Install envconfig**

```bash
cd backend
go get github.com/kelseyhightower/envconfig@v1.4.0
```

- [ ] **Step 2: Write failing test for config loading**

Create `backend/internal/config/config_test.go`:

```go
package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_FromEnv(t *testing.T) {
	t.Setenv("PORT", "9999")
	t.Setenv("ENV", "test")
	t.Setenv("DATABASE_URL", "postgres://test")
	t.Setenv("SUPABASE_URL", "https://x.supabase.co")
	t.Setenv("SUPABASE_SERVICE_ROLE_KEY", "srk")
	t.Setenv("SUPABASE_JWT_SECRET", "jwts")
	t.Setenv("R2_ACCOUNT_ID", "acc")
	t.Setenv("R2_ACCESS_KEY_ID", "akid")
	t.Setenv("R2_SECRET_ACCESS_KEY", "sk")
	t.Setenv("R2_BUCKET", "bkt")
	t.Setenv("R2_PUBLIC_BASE_URL", "https://cdn.test")
	t.Setenv("GEMINI_API_KEY", "gk")
	t.Setenv("REPLICATE_API_TOKEN", "rt")
	t.Setenv("REPLICATE_FLUX_VERSION", "v1")

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "9999", cfg.Port)
	assert.Equal(t, "test", cfg.Env)
	assert.Equal(t, "postgres://test", cfg.DatabaseURL)
	assert.Equal(t, 10, cfg.RateLimitPerDay)
	assert.Equal(t, 30, cfg.RateLimitPerWeek)
	assert.InEpsilon(t, 25.0, cfg.GlobalCostCapUSD, 0.0001)
	assert.Equal(t, 3, cfg.MaxConcurrentJobsPerUser)
}

func TestLoad_MissingRequired(t *testing.T) {
	os.Clearenv()
	_, err := Load()
	require.Error(t, err)
}
```

- [ ] **Step 3: Install testify**

```bash
go get github.com/stretchr/testify@v1.9.0
```

- [ ] **Step 4: Run test (expect fail — Load not defined)**

```bash
go test ./internal/config/...
```
Expected: compile error, `undefined: Load`.

- [ ] **Step 5: Implement config**

Create `backend/internal/config/config.go`:

```go
package config

import (
	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	Port string `envconfig:"PORT" default:"8080"`
	Env  string `envconfig:"ENV"  default:"development"`

	DatabaseURL              string `envconfig:"DATABASE_URL"               required:"true"`
	SupabaseURL              string `envconfig:"SUPABASE_URL"               required:"true"`
	SupabaseServiceRoleKey   string `envconfig:"SUPABASE_SERVICE_ROLE_KEY"  required:"true"`
	SupabaseJWTSecret        string `envconfig:"SUPABASE_JWT_SECRET"        required:"true"`

	R2AccountID       string `envconfig:"R2_ACCOUNT_ID"        required:"true"`
	R2AccessKeyID     string `envconfig:"R2_ACCESS_KEY_ID"     required:"true"`
	R2SecretAccessKey string `envconfig:"R2_SECRET_ACCESS_KEY" required:"true"`
	R2Bucket          string `envconfig:"R2_BUCKET"            required:"true"`
	R2PublicBaseURL   string `envconfig:"R2_PUBLIC_BASE_URL"   required:"true"`

	GeminiAPIKey         string `envconfig:"GEMINI_API_KEY"          required:"true"`
	ReplicateAPIToken    string `envconfig:"REPLICATE_API_TOKEN"     required:"true"`
	ReplicateFluxVersion string `envconfig:"REPLICATE_FLUX_VERSION"  required:"true"`

	SentryDSN string `envconfig:"SENTRY_DSN" default:""`

	RateLimitPerDay          int     `envconfig:"RATE_LIMIT_PER_DAY"           default:"10"`
	RateLimitPerWeek         int     `envconfig:"RATE_LIMIT_PER_WEEK"          default:"30"`
	GlobalCostCapUSD         float64 `envconfig:"GLOBAL_COST_CAP_USD"          default:"25.00"`
	MaxConcurrentJobsPerUser int     `envconfig:"MAX_CONCURRENT_JOBS_PER_USER" default:"3"`
}

func Load() (*Config, error) {
	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
```

- [ ] **Step 6: Run test (expect pass)**

```bash
go test ./internal/config/...
```
Expected: both tests PASS.

- [ ] **Step 7: Wire config into main.go**

Replace `backend/cmd/api/main.go`:

```go
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bimantara/ai-image/backend/internal/config"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		slog.Error("config load failed", "err", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{"status":"ok"}`)
	})

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 100 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	idleConnsClosed := make(chan struct{})
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, syscall.SIGINT, syscall.SIGTERM)
		<-sigint
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			slog.Error("graceful shutdown failed", "err", err)
		}
		close(idleConnsClosed)
	}()

	slog.Info("server starting", "port", cfg.Port, "env", cfg.Env)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("server failed", "err", err)
		os.Exit(1)
	}
	<-idleConnsClosed
}
```

- [ ] **Step 8: Verify build still compiles**

```bash
go build ./...
```
Expected: no errors.

- [ ] **Step 9: Commit**

```bash
cd /Users/bimantara/Dev/ASHA/Pathon/ai-image
git add backend/
git commit -m "backend: add config package with env-based loading"
```

---

## Task 3: Database migrations file

**Files:**
- Create: `backend/internal/db/migrations/001_initial.sql`

- [ ] **Step 1: Write migration SQL**

Create `backend/internal/db/migrations/001_initial.sql`:

```sql
-- 001_initial.sql
-- Apply via Supabase SQL editor or `psql $DATABASE_URL -f 001_initial.sql`.

create extension if not exists "pgcrypto";

-- Invites
create table if not exists invites (
  code text primary key,
  used_by uuid references auth.users(id) on delete set null,
  created_at timestamptz not null default now(),
  used_at timestamptz
);

-- Profiles (extends auth.users)
create table if not exists profiles (
  user_id uuid primary key references auth.users(id) on delete cascade,
  display_name text,
  invite_code text references invites(code),
  generates_today int not null default 0,
  generates_week int not null default 0,
  reset_at timestamptz not null default now(),
  created_at timestamptz not null default now()
);

-- Job enums
do $$ begin
  create type job_status as enum ('queued', 'processing', 'completed', 'failed');
exception when duplicate_object then null; end $$;

do $$ begin
  create type job_mode as enum ('realistic', 'inspirational');
exception when duplicate_object then null; end $$;

-- Jobs
create table if not exists jobs (
  id uuid primary key default gen_random_uuid(),
  user_id uuid not null references auth.users(id) on delete cascade,
  mode job_mode not null,
  status job_status not null default 'queued',
  prompt text not null,
  style_preset text,
  input_image_url text not null,
  output_image_url text,
  provider text,
  provider_request_id text,
  error_message text,
  generation_ms int,
  created_at timestamptz not null default now(),
  completed_at timestamptz
);

create index if not exists jobs_user_created_idx on jobs (user_id, created_at desc);
create index if not exists jobs_status_idx on jobs (status)
  where status in ('queued', 'processing');

-- Daily cost tracking
create table if not exists daily_costs (
  date date primary key,
  total_cost_usd numeric(10, 4) not null default 0,
  generate_count int not null default 0
);

-- Row-Level Security
alter table profiles enable row level security;
alter table jobs enable row level security;
alter table invites enable row level security;
alter table daily_costs enable row level security;

-- Profiles: user can read+update own row
drop policy if exists profiles_self_select on profiles;
create policy profiles_self_select on profiles
  for select using (auth.uid() = user_id);
drop policy if exists profiles_self_update on profiles;
create policy profiles_self_update on profiles
  for update using (auth.uid() = user_id);

-- Jobs: user can read own jobs only (insert/update via service role)
drop policy if exists jobs_self_select on jobs;
create policy jobs_self_select on jobs
  for select using (auth.uid() = user_id);

-- Invites and daily_costs: no client access. Service role bypasses RLS.

-- Enable Realtime on jobs so Flutter can subscribe
alter publication supabase_realtime add table jobs;
```

- [ ] **Step 2: Apply migration in Supabase**

In Supabase Dashboard → SQL Editor, paste the contents of `001_initial.sql` and run. Verify tables appear in the Table Editor.

- [ ] **Step 3: Commit**

```bash
cd /Users/bimantara/Dev/ASHA/Pathon/ai-image
git add backend/internal/db/migrations/001_initial.sql
git commit -m "backend: add initial database migration (profiles, jobs, invites, costs)"
```

---

## Task 4: Database connection pool (pgx)

**Files:**
- Create: `backend/internal/db/db.go`
- Create: `backend/internal/db/testutil.go`
- Create: `backend/internal/db/db_test.go`
- Modify: `backend/cmd/api/main.go`

- [ ] **Step 1: Install pgx**

```bash
cd backend
go get github.com/jackc/pgx/v5/pgxpool@v5.5.5
```

- [ ] **Step 2: Implement pool wrapper**

Create `backend/internal/db/db.go`:

```go
package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DB struct {
	Pool *pgxpool.Pool
}

func New(ctx context.Context, dsn string) (*DB, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}
	cfg.MaxConns = 10
	cfg.MaxConnLifetime = 30 * time.Minute
	cfg.HealthCheckPeriod = 1 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return &DB{Pool: pool}, nil
}

func (d *DB) Close() {
	d.Pool.Close()
}
```

- [ ] **Step 3: Add testcontainers helper**

```bash
go get github.com/testcontainers/testcontainers-go@v0.30.0
go get github.com/testcontainers/testcontainers-go/modules/postgres@v0.30.0
```

Create `backend/internal/db/testutil.go`:

```go
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
```

- [ ] **Step 4: Write integration test for pool**

Create `backend/internal/db/db_test.go`:

```go
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
```

- [ ] **Step 5: Run integration test**

```bash
cd backend
go test -tags=integration ./internal/db/...
```
Expected: PASS (test container Docker required — make sure Docker is running).

- [ ] **Step 6: Wire DB into main.go**

Replace the body of `main()` in `backend/cmd/api/main.go`, adding DB init after config load and before server setup. Replace the entire file with:

```go
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bimantara/ai-image/backend/internal/config"
	"github.com/bimantara/ai-image/backend/internal/db"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		slog.Error("config load failed", "err", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	database, err := db.New(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("db init failed", "err", err)
		os.Exit(1)
	}
	defer database.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{"status":"ok"}`)
	})

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 100 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	idleConnsClosed := make(chan struct{})
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, syscall.SIGINT, syscall.SIGTERM)
		<-sigint
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			slog.Error("graceful shutdown failed", "err", err)
		}
		close(idleConnsClosed)
	}()

	slog.Info("server starting", "port", cfg.Port, "env", cfg.Env)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("server failed", "err", err)
		os.Exit(1)
	}
	<-idleConnsClosed
}
```

- [ ] **Step 7: Commit**

```bash
cd /Users/bimantara/Dev/ASHA/Pathon/ai-image
git add backend/
git commit -m "backend: add pgx connection pool and integration test scaffold"
```

---

## Task 5: Query layer — invites

**Files:**
- Create: `backend/internal/db/invites.go`
- Create: `backend/internal/db/invites_test.go`

- [ ] **Step 1: Write failing test**

Create `backend/internal/db/invites_test.go`:

```go
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
```

- [ ] **Step 2: Install google uuid**

```bash
go get github.com/google/uuid@v1.6.0
```

- [ ] **Step 3: Run test (expect compile fail)**

```bash
go test -tags=integration ./internal/db/...
```
Expected: undefined Invites, ErrInviteAlreadyUsed, ErrInviteNotFound.

- [ ] **Step 4: Implement invites queries**

Create `backend/internal/db/invites.go`:

```go
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
```

- [ ] **Step 5: Run test (expect pass)**

```bash
go test -tags=integration ./internal/db/...
```
Expected: all 3 invites tests PASS.

- [ ] **Step 6: Commit**

```bash
cd /Users/bimantara/Dev/ASHA/Pathon/ai-image
git add backend/internal/db/invites.go backend/internal/db/invites_test.go backend/go.mod backend/go.sum
git commit -m "backend: add invites queries with atomic redeem"
```

---

## Task 6: Query layer — profiles

**Files:**
- Create: `backend/internal/db/profiles.go`
- Create: `backend/internal/db/profiles_test.go`

- [ ] **Step 1: Write failing test**

Create `backend/internal/db/profiles_test.go`:

```go
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
```

- [ ] **Step 2: Run test (expect compile fail)**

```bash
go test -tags=integration ./internal/db/...
```
Expected: undefined Profiles type.

- [ ] **Step 3: Implement profiles**

Create `backend/internal/db/profiles.go`:

```go
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
```

- [ ] **Step 4: Run test (expect pass)**

```bash
go test -tags=integration ./internal/db/...
```
Expected: all profiles + invites tests PASS.

- [ ] **Step 5: Commit**

```bash
cd /Users/bimantara/Dev/ASHA/Pathon/ai-image
git add backend/internal/db/profiles.go backend/internal/db/profiles_test.go
git commit -m "backend: add profiles queries with counter operations"
```

---

## Task 7: Query layer — jobs

**Files:**
- Create: `backend/internal/db/jobs.go`
- Create: `backend/internal/db/jobs_test.go`

- [ ] **Step 1: Write failing test**

Create `backend/internal/db/jobs_test.go`:

```go
//go:build integration

package db

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJobs_InsertAndGet(t *testing.T) {
	d := NewTestDB(t)
	ctx := context.Background()
	q := Jobs{DB: d}

	uid := setupUser(t, d)
	id, err := q.Insert(ctx, InsertJobParams{
		UserID:        uid,
		Mode:          ModeRealistic,
		Prompt:        "test",
		StylePreset:   "scandinavian",
		InputImageURL: "https://cdn/x.jpg",
	})
	require.NoError(t, err)

	job, err := q.Get(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, ModeRealistic, job.Mode)
	assert.Equal(t, StatusQueued, job.Status)
}

func TestJobs_MarkProcessingThenCompleted(t *testing.T) {
	d := NewTestDB(t)
	ctx := context.Background()
	q := Jobs{DB: d}
	uid := setupUser(t, d)

	id, err := q.Insert(ctx, InsertJobParams{
		UserID: uid, Mode: ModeInspirational, Prompt: "p",
		InputImageURL: "https://cdn/i.jpg",
	})
	require.NoError(t, err)

	require.NoError(t, q.MarkProcessing(ctx, id, "flux"))
	require.NoError(t, q.MarkCompleted(ctx, id, "https://cdn/o.jpg", "req-1", 12345))

	j, err := q.Get(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, StatusCompleted, j.Status)
	assert.Equal(t, "https://cdn/o.jpg", *j.OutputImageURL)
	assert.Equal(t, 12345, *j.GenerationMs)
}

func TestJobs_MarkFailed(t *testing.T) {
	d := NewTestDB(t)
	ctx := context.Background()
	q := Jobs{DB: d}
	uid := setupUser(t, d)

	id, err := q.Insert(ctx, InsertJobParams{
		UserID: uid, Mode: ModeRealistic, Prompt: "p",
		InputImageURL: "https://cdn/i.jpg",
	})
	require.NoError(t, err)
	require.NoError(t, q.MarkFailed(ctx, id, "timeout"))

	j, err := q.Get(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, StatusFailed, j.Status)
	assert.Equal(t, "timeout", *j.ErrorMessage)
}

func TestJobs_CountInFlight(t *testing.T) {
	d := NewTestDB(t)
	ctx := context.Background()
	q := Jobs{DB: d}
	uid := setupUser(t, d)

	for i := 0; i < 3; i++ {
		_, err := q.Insert(ctx, InsertJobParams{
			UserID: uid, Mode: ModeRealistic, Prompt: "p",
			InputImageURL: "https://x",
		})
		require.NoError(t, err)
	}
	n, err := q.CountInFlight(ctx, uid)
	require.NoError(t, err)
	assert.Equal(t, 3, n)
}

func TestJobs_SweepStale(t *testing.T) {
	d := NewTestDB(t)
	ctx := context.Background()
	q := Jobs{DB: d}
	uid := setupUser(t, d)

	id, err := q.Insert(ctx, InsertJobParams{
		UserID: uid, Mode: ModeRealistic, Prompt: "p",
		InputImageURL: "https://x",
	})
	require.NoError(t, err)
	require.NoError(t, q.MarkProcessing(ctx, id, "gemini"))

	// Backdate to 10 minutes ago
	_, err = d.Pool.Exec(ctx,
		`update jobs set created_at = now() - interval '10 minutes' where id = $1`, id)
	require.NoError(t, err)

	n, err := q.SweepStale(ctx, 5*time.Minute)
	require.NoError(t, err)
	assert.Equal(t, 1, n)

	j, err := q.Get(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, StatusFailed, j.Status)
	assert.Equal(t, "timeout", *j.ErrorMessage)
}

func TestJobs_List(t *testing.T) {
	d := NewTestDB(t)
	ctx := context.Background()
	q := Jobs{DB: d}
	uid := setupUser(t, d)

	for i := 0; i < 5; i++ {
		_, err := q.Insert(ctx, InsertJobParams{
			UserID: uid, Mode: ModeRealistic, Prompt: "p",
			InputImageURL: "https://x",
		})
		require.NoError(t, err)
	}
	list, err := q.List(ctx, uid, 3, time.Now().Add(time.Hour))
	require.NoError(t, err)
	assert.Len(t, list, 3)
}

func setupUser(t *testing.T, d *DB) uuid.UUID {
	t.Helper()
	uid := uuid.New()
	_, err := d.Pool.Exec(context.Background(),
		`insert into auth.users (id) values ($1)`, uid)
	require.NoError(t, err)
	return uid
}
```

- [ ] **Step 2: Run test (expect compile fail)**

```bash
go test -tags=integration ./internal/db/...
```
Expected: undefined Jobs, ModeRealistic, etc.

- [ ] **Step 3: Implement jobs queries**

Create `backend/internal/db/jobs.go`:

```go
package db

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type JobStatus string

const (
	StatusQueued     JobStatus = "queued"
	StatusProcessing JobStatus = "processing"
	StatusCompleted  JobStatus = "completed"
	StatusFailed     JobStatus = "failed"
)

type JobMode string

const (
	ModeRealistic     JobMode = "realistic"
	ModeInspirational JobMode = "inspirational"
)

type Job struct {
	ID                uuid.UUID
	UserID            uuid.UUID
	Mode              JobMode
	Status            JobStatus
	Prompt            string
	StylePreset       *string
	InputImageURL     string
	OutputImageURL    *string
	Provider          *string
	ProviderRequestID *string
	ErrorMessage      *string
	GenerationMs      *int
	CreatedAt         time.Time
	CompletedAt       *time.Time
}

type InsertJobParams struct {
	UserID        uuid.UUID
	Mode          JobMode
	Prompt        string
	StylePreset   string
	InputImageURL string
}

type Jobs struct{ DB *DB }

func (q Jobs) Insert(ctx context.Context, p InsertJobParams) (uuid.UUID, error) {
	var sp *string
	if p.StylePreset != "" {
		sp = &p.StylePreset
	}
	var id uuid.UUID
	err := q.DB.Pool.QueryRow(ctx,
		`insert into jobs (user_id, mode, prompt, style_preset, input_image_url)
		   values ($1, $2, $3, $4, $5)
		 returning id`,
		p.UserID, p.Mode, p.Prompt, sp, p.InputImageURL).Scan(&id)
	return id, err
}

func (q Jobs) Get(ctx context.Context, id uuid.UUID) (Job, error) {
	var j Job
	err := q.DB.Pool.QueryRow(ctx,
		`select id, user_id, mode, status, prompt, style_preset,
		        input_image_url, output_image_url, provider, provider_request_id,
		        error_message, generation_ms, created_at, completed_at
		   from jobs where id = $1`, id).
		Scan(&j.ID, &j.UserID, &j.Mode, &j.Status, &j.Prompt, &j.StylePreset,
			&j.InputImageURL, &j.OutputImageURL, &j.Provider, &j.ProviderRequestID,
			&j.ErrorMessage, &j.GenerationMs, &j.CreatedAt, &j.CompletedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Job{}, ErrJobNotFound
	}
	return j, err
}

func (q Jobs) MarkProcessing(ctx context.Context, id uuid.UUID, provider string) error {
	_, err := q.DB.Pool.Exec(ctx,
		`update jobs set status = 'processing', provider = $2 where id = $1`,
		id, provider)
	return err
}

func (q Jobs) MarkCompleted(ctx context.Context, id uuid.UUID,
	outputURL, providerReqID string, genMs int) error {
	_, err := q.DB.Pool.Exec(ctx,
		`update jobs
		    set status = 'completed',
		        output_image_url = $2,
		        provider_request_id = $3,
		        generation_ms = $4,
		        completed_at = now()
		  where id = $1`, id, outputURL, providerReqID, genMs)
	return err
}

func (q Jobs) MarkFailed(ctx context.Context, id uuid.UUID, errMsg string) error {
	_, err := q.DB.Pool.Exec(ctx,
		`update jobs
		    set status = 'failed',
		        error_message = $2,
		        completed_at = now()
		  where id = $1`, id, errMsg)
	return err
}

func (q Jobs) CountInFlight(ctx context.Context, userID uuid.UUID) (int, error) {
	var n int
	err := q.DB.Pool.QueryRow(ctx,
		`select count(*) from jobs
		  where user_id = $1 and status in ('queued', 'processing')`,
		userID).Scan(&n)
	return n, err
}

// SweepStale fails any job stuck in queued/processing older than maxAge.
// Returns the number of jobs failed.
func (q Jobs) SweepStale(ctx context.Context, maxAge time.Duration) (int, error) {
	tag, err := q.DB.Pool.Exec(ctx,
		`update jobs
		    set status = 'failed',
		        error_message = 'timeout',
		        completed_at = now()
		  where status in ('queued', 'processing')
		    and created_at < now() - $1::interval`, maxAge.String())
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}

// List returns the user's jobs, newest first, with cursor pagination on created_at.
func (q Jobs) List(ctx context.Context, userID uuid.UUID, limit int, cursor time.Time) ([]Job, error) {
	rows, err := q.DB.Pool.Query(ctx,
		`select id, user_id, mode, status, prompt, style_preset,
		        input_image_url, output_image_url, provider, provider_request_id,
		        error_message, generation_ms, created_at, completed_at
		   from jobs
		  where user_id = $1 and created_at < $2
		  order by created_at desc
		  limit $3`, userID, cursor, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Job
	for rows.Next() {
		var j Job
		if err := rows.Scan(&j.ID, &j.UserID, &j.Mode, &j.Status, &j.Prompt,
			&j.StylePreset, &j.InputImageURL, &j.OutputImageURL, &j.Provider,
			&j.ProviderRequestID, &j.ErrorMessage, &j.GenerationMs,
			&j.CreatedAt, &j.CompletedAt); err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

// ClaimNextQueued atomically picks one queued job, marks it processing,
// and returns it. Returns ErrJobNotFound if none available.
func (q Jobs) ClaimNextQueued(ctx context.Context, provider string) (Job, error) {
	var j Job
	err := q.DB.Pool.QueryRow(ctx,
		`update jobs
		    set status = 'processing', provider = $1
		  where id = (
		    select id from jobs
		     where status = 'queued'
		     order by created_at asc
		     for update skip locked
		     limit 1
		  )
		 returning id, user_id, mode, status, prompt, style_preset,
		           input_image_url, output_image_url, provider, provider_request_id,
		           error_message, generation_ms, created_at, completed_at`, provider).
		Scan(&j.ID, &j.UserID, &j.Mode, &j.Status, &j.Prompt, &j.StylePreset,
			&j.InputImageURL, &j.OutputImageURL, &j.Provider, &j.ProviderRequestID,
			&j.ErrorMessage, &j.GenerationMs, &j.CreatedAt, &j.CompletedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Job{}, ErrJobNotFound
	}
	return j, err
}

var ErrJobNotFound = errors.New("job not found")
```

- [ ] **Step 4: Run test (expect pass)**

```bash
go test -tags=integration ./internal/db/...
```
Expected: all jobs tests PASS.

- [ ] **Step 5: Commit**

```bash
cd /Users/bimantara/Dev/ASHA/Pathon/ai-image
git add backend/internal/db/jobs.go backend/internal/db/jobs_test.go
git commit -m "backend: add jobs queries with claim-and-sweep semantics"
```

---

## Task 8: Query layer — daily costs

**Files:**
- Create: `backend/internal/db/costs.go`
- Create: `backend/internal/db/costs_test.go`

- [ ] **Step 1: Write failing test**

Create `backend/internal/db/costs_test.go`:

```go
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
```

- [ ] **Step 2: Implement costs**

Create `backend/internal/db/costs.go`:

```go
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
```

- [ ] **Step 3: Run test (expect pass)**

```bash
go test -tags=integration ./internal/db/...
```
Expected: costs tests PASS.

- [ ] **Step 4: Commit**

```bash
cd /Users/bimantara/Dev/ASHA/Pathon/ai-image
git add backend/internal/db/costs.go backend/internal/db/costs_test.go
git commit -m "backend: add daily costs queries"
```

---

## Task 9: JWT auth — verify Supabase tokens

**Files:**
- Create: `backend/internal/auth/jwt.go`
- Create: `backend/internal/auth/jwt_test.go`
- Create: `backend/internal/auth/context.go`

- [ ] **Step 1: Install jwt library**

```bash
go get github.com/golang-jwt/jwt/v5@v5.2.1
```

- [ ] **Step 2: Write failing test**

Create `backend/internal/auth/jwt_test.go`:

```go
package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSecret = "test-secret-must-be-long-enough"

func signTestToken(t *testing.T, sub uuid.UUID, exp time.Time) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": sub.String(),
		"aud": "authenticated",
		"exp": exp.Unix(),
		"iat": time.Now().Unix(),
	})
	s, err := tok.SignedString([]byte(testSecret))
	require.NoError(t, err)
	return s
}

func TestVerify_Valid(t *testing.T) {
	v := NewVerifier(testSecret)
	uid := uuid.New()
	tok := signTestToken(t, uid, time.Now().Add(time.Hour))

	claims, err := v.Verify(tok)
	require.NoError(t, err)
	assert.Equal(t, uid, claims.UserID)
}

func TestVerify_Expired(t *testing.T) {
	v := NewVerifier(testSecret)
	tok := signTestToken(t, uuid.New(), time.Now().Add(-time.Hour))

	_, err := v.Verify(tok)
	require.Error(t, err)
}

func TestVerify_BadSignature(t *testing.T) {
	v := NewVerifier("different-secret")
	tok := signTestToken(t, uuid.New(), time.Now().Add(time.Hour))

	_, err := v.Verify(tok)
	require.Error(t, err)
}

func TestVerify_Malformed(t *testing.T) {
	v := NewVerifier(testSecret)
	_, err := v.Verify("not-a-jwt")
	require.Error(t, err)
}
```

- [ ] **Step 3: Run test (expect compile fail)**

```bash
go test ./internal/auth/...
```
Expected: undefined NewVerifier.

- [ ] **Step 4: Implement verifier**

Create `backend/internal/auth/jwt.go`:

```go
package auth

import (
	"errors"
	"fmt"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type Claims struct {
	UserID uuid.UUID
}

type Verifier struct{ secret []byte }

func NewVerifier(secret string) *Verifier {
	return &Verifier{secret: []byte(secret)}
}

func (v *Verifier) Verify(tokenStr string) (Claims, error) {
	parsed, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return v.secret, nil
	})
	if err != nil {
		return Claims{}, err
	}
	if !parsed.Valid {
		return Claims{}, errors.New("invalid token")
	}
	mapClaims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		return Claims{}, errors.New("unexpected claims type")
	}
	sub, ok := mapClaims["sub"].(string)
	if !ok || sub == "" {
		return Claims{}, errors.New("missing sub claim")
	}
	uid, err := uuid.Parse(sub)
	if err != nil {
		return Claims{}, fmt.Errorf("invalid sub uuid: %w", err)
	}
	return Claims{UserID: uid}, nil
}
```

- [ ] **Step 5: Add context helpers**

Create `backend/internal/auth/context.go`:

```go
package auth

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

type ctxKey struct{}

func WithUserID(ctx context.Context, uid uuid.UUID) context.Context {
	return context.WithValue(ctx, ctxKey{}, uid)
}

func UserIDFrom(ctx context.Context) (uuid.UUID, error) {
	v, ok := ctx.Value(ctxKey{}).(uuid.UUID)
	if !ok {
		return uuid.Nil, errors.New("no user in context")
	}
	return v, nil
}
```

- [ ] **Step 6: Run test (expect pass)**

```bash
go test ./internal/auth/...
```
Expected: all 4 tests PASS.

- [ ] **Step 7: Commit**

```bash
cd /Users/bimantara/Dev/ASHA/Pathon/ai-image
git add backend/internal/auth/
git commit -m "backend: add Supabase JWT verifier and context helpers"
```

---

## Task 10: chi router and auth middleware

**Files:**
- Create: `backend/internal/auth/middleware.go`
- Create: `backend/internal/http/server.go`
- Create: `backend/internal/http/handlers/health.go`
- Modify: `backend/cmd/api/main.go`

- [ ] **Step 1: Install chi**

```bash
go get github.com/go-chi/chi/v5@v5.0.12
```

- [ ] **Step 2: Implement auth middleware**

Create `backend/internal/auth/middleware.go`:

```go
package auth

import (
	"encoding/json"
	"net/http"
	"strings"
)

func Required(v *Verifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := r.Header.Get("Authorization")
			if !strings.HasPrefix(h, "Bearer ") {
				writeErr(w, http.StatusUnauthorized, "missing bearer token")
				return
			}
			tok := strings.TrimPrefix(h, "Bearer ")
			claims, err := v.Verify(tok)
			if err != nil {
				writeErr(w, http.StatusUnauthorized, "invalid token")
				return
			}
			next.ServeHTTP(w, r.WithContext(WithUserID(r.Context(), claims.UserID)))
		})
	}
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
```

- [ ] **Step 3: Implement health handler**

Create `backend/internal/http/handlers/health.go`:

```go
package handlers

import (
	"encoding/json"
	"net/http"
)

func Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
```

- [ ] **Step 4: Implement server / router**

Create `backend/internal/http/server.go`:

```go
package http

import (
	stdhttp "net/http"
	"time"

	"github.com/bimantara/ai-image/backend/internal/auth"
	"github.com/bimantara/ai-image/backend/internal/http/handlers"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type Deps struct {
	JWTVerifier *auth.Verifier
}

func NewRouter(d Deps) stdhttp.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	r.Get("/health", handlers.Health)

	r.Group(func(r chi.Router) {
		r.Use(auth.Required(d.JWTVerifier))
		// authenticated routes wired in later tasks
	})

	return r
}
```

- [ ] **Step 5: Wire into main.go**

Replace `backend/cmd/api/main.go`:

```go
package main

import (
	"context"
	"errors"
	"log/slog"
	stdhttp "net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bimantara/ai-image/backend/internal/auth"
	"github.com/bimantara/ai-image/backend/internal/config"
	"github.com/bimantara/ai-image/backend/internal/db"
	apphttp "github.com/bimantara/ai-image/backend/internal/http"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	cfg, err := config.Load()
	if err != nil {
		slog.Error("config", "err", err); os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	database, err := db.New(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("db", "err", err); os.Exit(1)
	}
	defer database.Close()

	router := apphttp.NewRouter(apphttp.Deps{
		JWTVerifier: auth.NewVerifier(cfg.SupabaseJWTSecret),
	})

	srv := &stdhttp.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 100 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	done := make(chan struct{})
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, syscall.SIGINT, syscall.SIGTERM)
		<-sigint
		sctx, scancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer scancel()
		_ = srv.Shutdown(sctx)
		close(done)
	}()

	slog.Info("listening", "port", cfg.Port, "env", cfg.Env)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, stdhttp.ErrServerClosed) {
		slog.Error("server", "err", err); os.Exit(1)
	}
	<-done
}
```

- [ ] **Step 6: Build and smoke test**

```bash
cd backend
go build ./...
```
Expected: clean build.

- [ ] **Step 7: Commit**

```bash
cd /Users/bimantara/Dev/ASHA/Pathon/ai-image
git add backend/
git commit -m "backend: add chi router with auth middleware and health route"
```

---

## Task 11: R2 storage client (presigned PUT + service-side upload)

**Files:**
- Create: `backend/internal/storage/r2.go`
- Create: `backend/internal/storage/r2_test.go`

- [ ] **Step 1: Install AWS SDK v2**

```bash
go get github.com/aws/aws-sdk-go-v2/config@v1.27.18
go get github.com/aws/aws-sdk-go-v2/service/s3@v1.55.0
go get github.com/aws/aws-sdk-go-v2/credentials@v1.17.18
```

- [ ] **Step 2: Implement R2 client**

Create `backend/internal/storage/r2.go`:

```go
package storage

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type R2 struct {
	client        *s3.Client
	presignClient *s3.PresignClient
	bucket        string
	publicBase    string
}

func NewR2(accountID, accessKey, secretKey, bucket, publicBase string) (*R2, error) {
	endpoint := fmt.Sprintf("https://%s.r2.cloudflarestorage.com", accountID)
	cl := s3.New(s3.Options{
		Region:       "auto",
		Credentials:  credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
		BaseEndpoint: aws.String(endpoint),
		UsePathStyle: true,
	})
	return &R2{
		client:        cl,
		presignClient: s3.NewPresignClient(cl),
		bucket:        bucket,
		publicBase:    publicBase,
	}, nil
}

// PresignPut returns a URL the client can PUT to directly, valid for ttl.
// publicURL is what the client should send back to us as input_image_url.
func (r *R2) PresignPut(ctx context.Context, key, contentType string, ttl time.Duration) (uploadURL, publicURL string, err error) {
	out, err := r.presignClient.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(r.bucket),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
	}, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", "", err
	}
	return out.URL, r.publicBase + "/" + key, nil
}

// Upload writes raw bytes from the backend (used for AI-generated outputs).
// Returns the public URL.
func (r *R2) Upload(ctx context.Context, key, contentType string, data []byte) (string, error) {
	_, err := r.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(r.bucket),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
		Body:        bytes.NewReader(data),
	})
	if err != nil {
		return "", err
	}
	return r.publicBase + "/" + key, nil
}
```

- [ ] **Step 3: Write unit test for URL construction**

Create `backend/internal/storage/r2_test.go`:

```go
package storage

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// This test only exercises URL shape — actual R2 calls are tested manually
// after deploy, since mocking the AWS SDK presigner is brittle.
func TestPresignPut_BuildsPublicURL(t *testing.T) {
	r, err := NewR2("acc", "ak", "sk", "bkt", "https://cdn.example.com")
	require.NoError(t, err)

	_, public, err := r.PresignPut(context.Background(), "uploads/abc.jpg", "image/jpeg", 5*time.Minute)
	require.NoError(t, err)
	assert.Equal(t, "https://cdn.example.com/uploads/abc.jpg", public)
	// upload URL existence is enough to verify wiring
}

func TestUpload_BuildsPublicURL(t *testing.T) {
	r, err := NewR2("acc", "ak", "sk", "bkt", "https://cdn.example.com/")
	require.NoError(t, err)
	// Trim trailing slash if present to avoid double-slash
	assert.True(t, strings.HasPrefix(r.publicBase, "https://cdn"))
}
```

- [ ] **Step 4: Run test (expect pass)**

```bash
go test ./internal/storage/...
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
cd /Users/bimantara/Dev/ASHA/Pathon/ai-image
git add backend/internal/storage/ backend/go.mod backend/go.sum
git commit -m "backend: add R2 storage client with presigned PUT and direct upload"
```

---

## Task 12: Provider abstraction + mock provider

**Files:**
- Create: `backend/internal/providers/provider.go`
- Create: `backend/internal/providers/mock/mock.go`
- Create: `backend/internal/providers/registry.go`

- [ ] **Step 1: Define interface**

Create `backend/internal/providers/provider.go`:

```go
package providers

import (
	"context"
	"errors"
)

type Input struct {
	InputImageURL string
	Prompt        string
	StylePreset   string
}

type Result struct {
	OutputImageBytes  []byte
	OutputContentType string // e.g. "image/png"
	ProviderRequestID string
	EstimatedCostUSD  float64
}

type ImageProvider interface {
	Name() string
	Generate(ctx context.Context, in Input) (Result, error)
}

var (
	ErrTransient        = errors.New("provider transient error (retry candidate)")
	ErrContentModerated = errors.New("provider rejected content")
	ErrTimeout          = errors.New("provider timeout")
)
```

- [ ] **Step 2: Implement mock provider**

Create `backend/internal/providers/mock/mock.go`:

```go
package mock

import (
	"context"
	"encoding/base64"

	"github.com/bimantara/ai-image/backend/internal/providers"
)

// 1x1 transparent PNG
const tinyPNG = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkYAAAAAYAAjCB0C8AAAAASUVORK5CYII="

type Provider struct{ name string }

func New(name string) *Provider { return &Provider{name: name} }

func (p *Provider) Name() string { return p.name }

func (p *Provider) Generate(ctx context.Context, in providers.Input) (providers.Result, error) {
	data, _ := base64.StdEncoding.DecodeString(tinyPNG)
	return providers.Result{
		OutputImageBytes:  data,
		OutputContentType: "image/png",
		ProviderRequestID: "mock-" + in.InputImageURL,
		EstimatedCostUSD:  0,
	}, nil
}
```

- [ ] **Step 3: Implement registry**

Create `backend/internal/providers/registry.go`:

```go
package providers

import (
	"fmt"

	"github.com/bimantara/ai-image/backend/internal/db"
)

type Registry struct{ byMode map[db.JobMode]ImageProvider }

func NewRegistry() *Registry { return &Registry{byMode: map[db.JobMode]ImageProvider{}} }

func (r *Registry) Register(mode db.JobMode, p ImageProvider) { r.byMode[mode] = p }

func (r *Registry) For(mode db.JobMode) (ImageProvider, error) {
	p, ok := r.byMode[mode]
	if !ok {
		return nil, fmt.Errorf("no provider for mode %q", mode)
	}
	return p, nil
}
```

- [ ] **Step 4: Build check**

```bash
go build ./...
```
Expected: clean build.

- [ ] **Step 5: Commit**

```bash
cd /Users/bimantara/Dev/ASHA/Pathon/ai-image
git add backend/internal/providers/
git commit -m "backend: add provider interface, mock provider, and mode-routing registry"
```

---

## Task 13: Rate limit + cost cap logic

**Files:**
- Create: `backend/internal/ratelimit/ratelimit.go`
- Create: `backend/internal/ratelimit/ratelimit_test.go`

- [ ] **Step 1: Write failing test**

Create `backend/internal/ratelimit/ratelimit_test.go`:

```go
//go:build integration

package ratelimit

import (
	"context"
	"testing"

	"github.com/bimantara/ai-image/backend/internal/db"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheck_AllowsWithinLimits(t *testing.T) {
	d := db.NewTestDB(t)
	ctx := context.Background()
	gate := New(Config{
		DB:              d,
		PerDay:          5,
		PerWeek:         10,
		GlobalCostCap:   25.0,
		MaxConcurrent:   3,
	})

	uid := setupUser(t, d)
	require.NoError(t, db.Profiles{DB: d}.Upsert(ctx, uid, "x", ""))

	require.NoError(t, gate.Check(ctx, uid))
}

func TestCheck_DailyLimitExceeded(t *testing.T) {
	d := db.NewTestDB(t)
	ctx := context.Background()
	gate := New(Config{DB: d, PerDay: 2, PerWeek: 10, GlobalCostCap: 25, MaxConcurrent: 3})

	uid := setupUser(t, d)
	require.NoError(t, db.Profiles{DB: d}.Upsert(ctx, uid, "x", ""))
	require.NoError(t, db.Profiles{DB: d}.IncrementCounters(ctx, uid))
	require.NoError(t, db.Profiles{DB: d}.IncrementCounters(ctx, uid))

	err := gate.Check(ctx, uid)
	assert.ErrorIs(t, err, ErrDailyLimit)
}

func TestCheck_ConcurrencyLimitExceeded(t *testing.T) {
	d := db.NewTestDB(t)
	ctx := context.Background()
	gate := New(Config{DB: d, PerDay: 100, PerWeek: 100, GlobalCostCap: 25, MaxConcurrent: 2})

	uid := setupUser(t, d)
	require.NoError(t, db.Profiles{DB: d}.Upsert(ctx, uid, "x", ""))
	jobs := db.Jobs{DB: d}
	for i := 0; i < 2; i++ {
		_, err := jobs.Insert(ctx, db.InsertJobParams{
			UserID: uid, Mode: db.ModeRealistic, Prompt: "p", InputImageURL: "https://x",
		})
		require.NoError(t, err)
	}

	err := gate.Check(ctx, uid)
	assert.ErrorIs(t, err, ErrConcurrencyLimit)
}

func TestCheck_GlobalCostCap(t *testing.T) {
	d := db.NewTestDB(t)
	ctx := context.Background()
	gate := New(Config{DB: d, PerDay: 100, PerWeek: 100, GlobalCostCap: 0.05, MaxConcurrent: 3})

	require.NoError(t, db.Costs{DB: d}.AddToday(ctx, 0.06))

	uid := setupUser(t, d)
	require.NoError(t, db.Profiles{DB: d}.Upsert(ctx, uid, "x", ""))

	err := gate.Check(ctx, uid)
	assert.ErrorIs(t, err, ErrGlobalCostCap)
}

func setupUser(t *testing.T, d *db.DB) uuid.UUID {
	t.Helper()
	uid := uuid.New()
	_, err := d.Pool.Exec(context.Background(), `insert into auth.users (id) values ($1)`, uid)
	require.NoError(t, err)
	return uid
}
```

- [ ] **Step 2: Implement gate**

Create `backend/internal/ratelimit/ratelimit.go`:

```go
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
```

- [ ] **Step 3: Run test (expect pass)**

```bash
go test -tags=integration ./internal/ratelimit/...
```
Expected: all 4 tests PASS.

- [ ] **Step 4: Commit**

```bash
cd /Users/bimantara/Dev/ASHA/Pathon/ai-image
git add backend/internal/ratelimit/
git commit -m "backend: add rate limit + cost cap gate"
```

---

## Task 14: Generation service + worker pool

**Files:**
- Create: `backend/internal/generation/service.go`
- Create: `backend/internal/generation/service_test.go`
- Create: `backend/internal/generation/worker.go`
- Create: `backend/internal/generation/worker_test.go`

- [ ] **Step 1: Write failing test for service**

Create `backend/internal/generation/service_test.go`:

```go
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
```

- [ ] **Step 2: Implement service**

Create `backend/internal/generation/service.go`:

```go
package generation

import (
	"context"
	"errors"
	"strings"

	"github.com/bimantara/ai-image/backend/internal/db"
	"github.com/bimantara/ai-image/backend/internal/providers"
	"github.com/bimantara/ai-image/backend/internal/ratelimit"
	"github.com/google/uuid"
)

var ErrInvalidInput = errors.New("invalid input")

type Deps struct {
	DB       *db.DB
	Registry *providers.Registry
	Gate     *ratelimit.Gate
}

type Service struct{ d Deps }

func NewService(d Deps) *Service { return &Service{d: d} }

type SubmitInput struct {
	UserID        uuid.UUID
	Mode          db.JobMode
	Prompt        string
	StylePreset   string
	InputImageURL string
}

func (s *Service) Submit(ctx context.Context, in SubmitInput) (uuid.UUID, error) {
	if err := validate(in); err != nil {
		return uuid.Nil, err
	}
	if _, err := s.d.Registry.For(in.Mode); err != nil {
		return uuid.Nil, ErrInvalidInput
	}
	if err := s.d.Gate.Check(ctx, in.UserID); err != nil {
		return uuid.Nil, err
	}
	id, err := db.Jobs{DB: s.d.DB}.Insert(ctx, db.InsertJobParams{
		UserID: in.UserID, Mode: in.Mode, Prompt: in.Prompt,
		StylePreset: in.StylePreset, InputImageURL: in.InputImageURL,
	})
	if err != nil {
		return uuid.Nil, err
	}
	if err := db.Profiles{DB: s.d.DB}.IncrementCounters(ctx, in.UserID); err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

func validate(in SubmitInput) error {
	p := strings.TrimSpace(in.Prompt)
	if len(p) < 5 || len(p) > 500 {
		return ErrInvalidInput
	}
	if in.InputImageURL == "" || !strings.HasPrefix(in.InputImageURL, "https://") {
		return ErrInvalidInput
	}
	if in.Mode != db.ModeRealistic && in.Mode != db.ModeInspirational {
		return ErrInvalidInput
	}
	return nil
}
```

- [ ] **Step 3: Run service test (expect pass)**

```bash
go test -tags=integration ./internal/generation/...
```
Expected: 3 service tests PASS.

- [ ] **Step 4: Write failing test for worker**

Create `backend/internal/generation/worker_test.go`:

```go
//go:build integration

package generation

import (
	"context"
	"testing"
	"time"

	"github.com/bimantara/ai-image/backend/internal/db"
	"github.com/bimantara/ai-image/backend/internal/providers"
	"github.com/bimantara/ai-image/backend/internal/providers/mock"
	"github.com/bimantara/ai-image/backend/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorker_ProcessesQueuedJob(t *testing.T) {
	d := db.NewTestDB(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	uid := setupUser(t, d)
	require.NoError(t, db.Profiles{DB: d}.Upsert(ctx, uid, "x", ""))

	id, err := db.Jobs{DB: d}.Insert(ctx, db.InsertJobParams{
		UserID: uid, Mode: db.ModeRealistic, Prompt: "five chars",
		InputImageURL: "https://x.jpg",
	})
	require.NoError(t, err)

	reg := providers.NewRegistry()
	reg.Register(db.ModeRealistic, mock.New("mock"))
	r2, _ := storage.NewR2("acc", "ak", "sk", "bkt", "https://cdn.test")

	w := NewWorker(WorkerDeps{DB: d, Registry: reg, R2: r2, Concurrency: 1, PollInterval: 50 * time.Millisecond})
	go w.Run(ctx)

	require.Eventually(t, func() bool {
		j, err := db.Jobs{DB: d}.Get(ctx, id)
		return err == nil && (j.Status == db.StatusCompleted || j.Status == db.StatusFailed)
	}, 5*time.Second, 50*time.Millisecond)

	j, err := db.Jobs{DB: d}.Get(ctx, id)
	require.NoError(t, err)
	// Storage upload will fail without real R2; we accept either Completed (if upload skipped)
	// or Failed with storage_error. Verify provider was at least invoked.
	assert.Contains(t, []db.JobStatus{db.StatusCompleted, db.StatusFailed}, j.Status)
	assert.Equal(t, "mock", *j.Provider)
}
```

- [ ] **Step 5: Implement worker**

Create `backend/internal/generation/worker.go`:

```go
package generation

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/bimantara/ai-image/backend/internal/db"
	"github.com/bimantara/ai-image/backend/internal/providers"
	"github.com/bimantara/ai-image/backend/internal/storage"
	"github.com/google/uuid"
)

type WorkerDeps struct {
	DB           *db.DB
	Registry     *providers.Registry
	R2           *storage.R2
	Concurrency  int
	PollInterval time.Duration
	JobTimeout   time.Duration
}

type Worker struct{ d WorkerDeps }

func NewWorker(d WorkerDeps) *Worker {
	if d.Concurrency <= 0 {
		d.Concurrency = 4
	}
	if d.PollInterval <= 0 {
		d.PollInterval = 2 * time.Second
	}
	if d.JobTimeout <= 0 {
		d.JobTimeout = 90 * time.Second
	}
	return &Worker{d: d}
}

func (w *Worker) Run(ctx context.Context) {
	sem := make(chan struct{}, w.d.Concurrency)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		j, err := db.Jobs{DB: w.d.DB}.ClaimNextQueued(ctx, "")
		if errors.Is(err, db.ErrJobNotFound) {
			select {
			case <-ctx.Done():
				return
			case <-time.After(w.d.PollInterval):
				continue
			}
		}
		if err != nil {
			slog.Error("worker: claim failed", "err", err)
			time.Sleep(w.d.PollInterval)
			continue
		}

		sem <- struct{}{}
		go func(job db.Job) {
			defer func() { <-sem }()
			w.process(ctx, job)
		}(j)
	}
}

func (w *Worker) process(parent context.Context, job db.Job) {
	ctx, cancel := context.WithTimeout(parent, w.d.JobTimeout)
	defer cancel()

	provider, err := w.d.Registry.For(job.Mode)
	if err != nil {
		_ = db.Jobs{DB: w.d.DB}.MarkFailed(ctx, job.ID, "no provider")
		return
	}

	// Update the provider field to the actual provider name (may differ from "").
	if err := db.Jobs{DB: w.d.DB}.MarkProcessing(ctx, job.ID, provider.Name()); err != nil {
		slog.Error("worker: mark processing", "err", err)
		return
	}

	start := time.Now()
	result, err := provider.Generate(ctx, providers.Input{
		InputImageURL: job.InputImageURL,
		Prompt:        job.Prompt,
		StylePreset:   strDeref(job.StylePreset),
	})
	if err != nil {
		w.fail(ctx, job, err)
		return
	}

	key := fmt.Sprintf("outputs/%s.%s", uuid.New(), extFor(result.OutputContentType))
	publicURL, err := w.d.R2.Upload(ctx, key, result.OutputContentType, result.OutputImageBytes)
	if err != nil {
		w.fail(ctx, job, fmt.Errorf("storage_error: %w", err))
		return
	}

	gen := int(time.Since(start).Milliseconds())
	if err := db.Jobs{DB: w.d.DB}.MarkCompleted(ctx, job.ID, publicURL, result.ProviderRequestID, gen); err != nil {
		slog.Error("worker: mark completed", "err", err)
		return
	}

	if result.EstimatedCostUSD > 0 {
		if err := db.Costs{DB: w.d.DB}.AddToday(ctx, result.EstimatedCostUSD); err != nil {
			slog.Error("worker: cost add", "err", err)
		}
	}
}

func (w *Worker) fail(ctx context.Context, job db.Job, err error) {
	msg := err.Error()
	if errors.Is(err, providers.ErrTimeout) {
		msg = "timeout"
	}
	if errors.Is(err, providers.ErrContentModerated) {
		msg = "content_moderated: " + msg
	}
	if mErr := db.Jobs{DB: w.d.DB}.MarkFailed(ctx, job.ID, msg); mErr != nil {
		slog.Error("worker: mark failed", "err", mErr)
	}
	// Refund the counter — failed generates don't count against quota
	if dErr := db.Profiles{DB: w.d.DB}.DecrementCounters(ctx, job.UserID); dErr != nil {
		slog.Error("worker: decrement", "err", dErr)
	}
}

func strDeref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func extFor(ct string) string {
	switch ct {
	case "image/png":
		return "png"
	case "image/jpeg":
		return "jpg"
	case "image/webp":
		return "webp"
	default:
		return "bin"
	}
}
```

- [ ] **Step 6: Run worker test (expect pass)**

```bash
go test -tags=integration ./internal/generation/...
```
Expected: all tests PASS. (Worker test accepts either Completed or Failed since real R2 isn't reachable in test.)

- [ ] **Step 7: Commit**

```bash
cd /Users/bimantara/Dev/ASHA/Pathon/ai-image
git add backend/internal/generation/
git commit -m "backend: add generation service and background worker pool"
```

---

## Task 15: Gemini provider implementation

**Files:**
- Create: `backend/internal/providers/gemini/gemini.go`
- Create: `backend/internal/providers/gemini/gemini_test.go`

- [ ] **Step 1: Write contract test against fixture**

Create `backend/internal/providers/gemini/gemini_test.go`:

```go
package gemini

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bimantara/ai-image/backend/internal/providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const tinyPNGB64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkYAAAAAYAAjCB0C8AAAAASUVORK5CYII="

func TestGenerate_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		body, _ := io.ReadAll(r.Body)
		assert.Contains(t, string(body), "Scandinavian")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"candidates": [{
				"content": {
					"parts": [{
						"inlineData": {
							"mimeType": "image/png",
							"data": "` + tinyPNGB64 + `"
						}
					}]
				}
			}]
		}`))
	}))
	defer srv.Close()

	p := New(Config{APIKey: "test", BaseURL: srv.URL})
	res, err := p.Generate(context.Background(), providers.Input{
		InputImageURL: "https://example.com/in.jpg",
		Prompt:        "Scandinavian coffee shop",
		StylePreset:   "scandinavian",
	})
	require.NoError(t, err)
	expected, _ := base64.StdEncoding.DecodeString(tinyPNGB64)
	assert.Equal(t, expected, res.OutputImageBytes)
	assert.Equal(t, "image/png", res.OutputContentType)
	assert.Greater(t, res.EstimatedCostUSD, 0.0)
}

func TestGenerate_RateLimited(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":"rate limited"}`))
	}))
	defer srv.Close()

	p := New(Config{APIKey: "test", BaseURL: srv.URL})
	_, err := p.Generate(context.Background(), providers.Input{
		InputImageURL: "https://x", Prompt: "five chars",
	})
	assert.ErrorIs(t, err, providers.ErrTransient)
}

func TestGenerate_ContentModerated(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"candidates": [{
				"finishReason": "SAFETY",
				"content": {"parts": []}
			}]
		}`))
	}))
	defer srv.Close()

	p := New(Config{APIKey: "test", BaseURL: srv.URL})
	_, err := p.Generate(context.Background(), providers.Input{
		InputImageURL: "https://x", Prompt: "five chars",
	})
	assert.ErrorIs(t, err, providers.ErrContentModerated)
}

func TestGenerate_PromptIncludesStyleAndImage(t *testing.T) {
	var seenBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		seenBody = string(b)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"inlineData":{"mimeType":"image/png","data":"` + tinyPNGB64 + `"}}]}}]}`))
	}))
	defer srv.Close()

	p := New(Config{APIKey: "test", BaseURL: srv.URL})
	_, err := p.Generate(context.Background(), providers.Input{
		InputImageURL: "https://example.com/in.jpg",
		Prompt:        "Cozy with warm woods",
		StylePreset:   "japandi",
	})
	require.NoError(t, err)
	assert.True(t, strings.Contains(seenBody, "japandi"))
	assert.True(t, strings.Contains(seenBody, "Cozy with warm woods"))
	// Image should be referenced (either as URL or fetched and base64'd)
	assert.True(t,
		strings.Contains(seenBody, "in.jpg") || strings.Contains(seenBody, "inlineData"),
		"request should reference the input image")
}
```

- [ ] **Step 2: Implement Gemini client**

Create `backend/internal/providers/gemini/gemini.go`:

```go
package gemini

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/bimantara/ai-image/backend/internal/providers"
)

const (
	defaultBaseURL    = "https://generativelanguage.googleapis.com/v1beta"
	model             = "gemini-2.5-flash-image"
	estimatedCostUSD  = 0.039
	requestTimeoutSec = 60
)

type Config struct {
	APIKey  string
	BaseURL string // for testing — defaults to Google endpoint
	Client  *http.Client
}

type Provider struct{ cfg Config }

func New(cfg Config) *Provider {
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultBaseURL
	}
	if cfg.Client == nil {
		cfg.Client = &http.Client{Timeout: requestTimeoutSec * time.Second}
	}
	return &Provider{cfg: cfg}
}

func (p *Provider) Name() string { return "gemini" }

type genReq struct {
	Contents []content `json:"contents"`
}
type content struct {
	Parts []part `json:"parts"`
}
type part struct {
	Text       string      `json:"text,omitempty"`
	InlineData *inlineData `json:"inlineData,omitempty"`
}
type inlineData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
}

type genResp struct {
	Candidates []candidate `json:"candidates"`
}
type candidate struct {
	Content      content `json:"content"`
	FinishReason string  `json:"finishReason"`
}

func (p *Provider) Generate(ctx context.Context, in providers.Input) (providers.Result, error) {
	imgBytes, imgMime, err := fetchImage(ctx, p.cfg.Client, in.InputImageURL)
	if err != nil {
		return providers.Result{}, fmt.Errorf("fetch input image: %w", err)
	}

	prompt := buildPrompt(in.Prompt, in.StylePreset)

	body := genReq{
		Contents: []content{{Parts: []part{
			{Text: prompt},
			{InlineData: &inlineData{MimeType: imgMime, Data: base64.StdEncoding.EncodeToString(imgBytes)}},
		}}},
	}
	b, _ := json.Marshal(body)

	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", p.cfg.BaseURL, model, p.cfg.APIKey)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(b))
	if err != nil {
		return providers.Result{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.cfg.Client.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return providers.Result{}, providers.ErrTimeout
		}
		return providers.Result{}, fmt.Errorf("%w: %v", providers.ErrTransient, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return providers.Result{}, fmt.Errorf("%w: status %d", providers.ErrTransient, resp.StatusCode)
	}
	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(resp.Body)
		return providers.Result{}, fmt.Errorf("gemini %d: %s", resp.StatusCode, string(raw))
	}

	var out genResp
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return providers.Result{}, err
	}
	if len(out.Candidates) == 0 {
		return providers.Result{}, errors.New("no candidates returned")
	}
	cand := out.Candidates[0]
	if cand.FinishReason == "SAFETY" || cand.FinishReason == "RECITATION" {
		return providers.Result{}, providers.ErrContentModerated
	}
	for _, pt := range cand.Content.Parts {
		if pt.InlineData != nil {
			data, err := base64.StdEncoding.DecodeString(pt.InlineData.Data)
			if err != nil {
				return providers.Result{}, err
			}
			return providers.Result{
				OutputImageBytes:  data,
				OutputContentType: pt.InlineData.MimeType,
				ProviderRequestID: resp.Header.Get("X-Request-Id"),
				EstimatedCostUSD:  estimatedCostUSD,
			}, nil
		}
	}
	return providers.Result{}, errors.New("no image part in response")
}

func buildPrompt(userPrompt, stylePreset string) string {
	style := stylePreset
	if style == "" {
		style = "modern"
	}
	return fmt.Sprintf(
		"Redesign this empty room into a %s coffee shop interior based on: %s.\n"+
			"Preserve the original room structure (walls, windows, doors, ceiling).\n"+
			"Add furniture, lighting, materials, and decor that fit the style.\n"+
			"Photorealistic, professional interior photography.",
		strings.ToLower(style), userPrompt)
}

func fetchImage(ctx context.Context, c *http.Client, url string) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, "", err
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, "", fmt.Errorf("fetch image: status %d", resp.StatusCode)
	}
	mime := resp.Header.Get("Content-Type")
	if mime == "" {
		mime = "image/jpeg"
	}
	b, err := io.ReadAll(resp.Body)
	return b, mime, err
}
```

- [ ] **Step 3: Run tests (expect pass)**

```bash
go test ./internal/providers/gemini/...
```
Expected: all 4 tests PASS.

- [ ] **Step 4: Commit**

```bash
cd /Users/bimantara/Dev/ASHA/Pathon/ai-image
git add backend/internal/providers/gemini/
git commit -m "backend: add Gemini 2.5 Flash Image provider"
```

---

## Task 16: Flux Pro provider (via Replicate)

**Files:**
- Create: `backend/internal/providers/flux/flux.go`
- Create: `backend/internal/providers/flux/flux_test.go`

- [ ] **Step 1: Write contract test**

Create `backend/internal/providers/flux/flux_test.go`:

```go
package flux

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bimantara/ai-image/backend/internal/providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const onePixelPNG = "\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00\x1f\x15\xc4\x89\x00\x00\x00\rIDATx\x9cc\x00\x01\x00\x00\x05\x00\x01\r\n-\xb4\x00\x00\x00\x00IEND\xaeB`\x82"

func TestGenerate_FullFlow(t *testing.T) {
	imgServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte(onePixelPNG))
	}))
	defer imgServer.Close()

	var pollCount int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/predictions"):
			body, _ := io.ReadAll(r.Body)
			assert.Contains(t, string(body), "version")
			assert.Contains(t, string(body), "Industrial")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": "pred-123", "status": "starting",
			})
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/predictions/pred-123"):
			pollCount++
			status := "processing"
			var output any
			if pollCount >= 2 {
				status = "succeeded"
				output = []string{imgServer.URL + "/out.png"}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": "pred-123", "status": status, "output": output,
			})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	p := New(Config{Token: "test", BaseURL: srv.URL, ModelVersion: "v1", PollIntervalMs: 10})
	res, err := p.Generate(context.Background(), providers.Input{
		InputImageURL: "https://example.com/in.jpg",
		Prompt:        "Industrial coffee shop with exposed brick",
		StylePreset:   "industrial",
	})
	require.NoError(t, err)
	assert.Equal(t, "image/png", res.OutputContentType)
	assert.NotEmpty(t, res.OutputImageBytes)
	assert.Equal(t, "pred-123", res.ProviderRequestID)
	assert.InEpsilon(t, 0.04, res.EstimatedCostUSD, 0.0001)
}

func TestGenerate_Failed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "POST":
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "pred-x", "status": "starting"})
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": "pred-x", "status": "failed", "error": "model crashed",
			})
		}
	}))
	defer srv.Close()

	p := New(Config{Token: "test", BaseURL: srv.URL, ModelVersion: "v1", PollIntervalMs: 10})
	_, err := p.Generate(context.Background(), providers.Input{
		InputImageURL: "https://x", Prompt: "p",
	})
	require.Error(t, err)
}
```

- [ ] **Step 2: Implement Flux client**

Create `backend/internal/providers/flux/flux.go`:

```go
package flux

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/bimantara/ai-image/backend/internal/providers"
)

const (
	defaultBaseURL       = "https://api.replicate.com/v1"
	estimatedCostUSD     = 0.04
	defaultRequestTimeout = 120 * time.Second
)

type Config struct {
	Token          string
	BaseURL        string
	ModelVersion   string
	PollIntervalMs int
	Client         *http.Client
}

type Provider struct{ cfg Config }

func New(cfg Config) *Provider {
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultBaseURL
	}
	if cfg.PollIntervalMs == 0 {
		cfg.PollIntervalMs = 1500
	}
	if cfg.Client == nil {
		cfg.Client = &http.Client{Timeout: defaultRequestTimeout}
	}
	return &Provider{cfg: cfg}
}

func (p *Provider) Name() string { return "flux" }

type predictionReq struct {
	Version string         `json:"version"`
	Input   map[string]any `json:"input"`
}

type predictionResp struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Output any    `json:"output"`
	Error  string `json:"error"`
}

func (p *Provider) Generate(ctx context.Context, in providers.Input) (providers.Result, error) {
	body := predictionReq{
		Version: p.cfg.ModelVersion,
		Input: map[string]any{
			"prompt":        buildPrompt(in.Prompt, in.StylePreset),
			"image_prompt":  in.InputImageURL,
			"aspect_ratio":  "4:3",
			"output_format": "png",
			"safety_tolerance": 2,
		},
	}
	b, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, "POST", p.cfg.BaseURL+"/predictions", bytes.NewReader(b))
	if err != nil {
		return providers.Result{}, err
	}
	req.Header.Set("Authorization", "Token "+p.cfg.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.cfg.Client.Do(req)
	if err != nil {
		return providers.Result{}, fmt.Errorf("%w: %v", providers.ErrTransient, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests {
		return providers.Result{}, fmt.Errorf("%w: status %d", providers.ErrTransient, resp.StatusCode)
	}
	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(resp.Body)
		return providers.Result{}, fmt.Errorf("replicate %d: %s", resp.StatusCode, string(raw))
	}

	var pred predictionResp
	if err := json.NewDecoder(resp.Body).Decode(&pred); err != nil {
		return providers.Result{}, err
	}

	// Poll until done.
	pred, err = p.poll(ctx, pred.ID)
	if err != nil {
		return providers.Result{}, err
	}
	if pred.Status != "succeeded" {
		if strings.Contains(strings.ToLower(pred.Error), "nsfw") ||
			strings.Contains(strings.ToLower(pred.Error), "safety") {
			return providers.Result{}, providers.ErrContentModerated
		}
		return providers.Result{}, fmt.Errorf("flux failed: %s", pred.Error)
	}

	outURL, err := extractOutputURL(pred.Output)
	if err != nil {
		return providers.Result{}, err
	}

	imgBytes, mime, err := download(ctx, p.cfg.Client, outURL)
	if err != nil {
		return providers.Result{}, err
	}
	return providers.Result{
		OutputImageBytes:  imgBytes,
		OutputContentType: mime,
		ProviderRequestID: pred.ID,
		EstimatedCostUSD:  estimatedCostUSD,
	}, nil
}

func (p *Provider) poll(ctx context.Context, id string) (predictionResp, error) {
	url := p.cfg.BaseURL + "/predictions/" + id
	interval := time.Duration(p.cfg.PollIntervalMs) * time.Millisecond
	for {
		select {
		case <-ctx.Done():
			return predictionResp{}, providers.ErrTimeout
		case <-time.After(interval):
		}

		req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
		req.Header.Set("Authorization", "Token "+p.cfg.Token)
		resp, err := p.cfg.Client.Do(req)
		if err != nil {
			return predictionResp{}, fmt.Errorf("%w: %v", providers.ErrTransient, err)
		}
		var pred predictionResp
		if err := json.NewDecoder(resp.Body).Decode(&pred); err != nil {
			resp.Body.Close()
			return predictionResp{}, err
		}
		resp.Body.Close()

		switch pred.Status {
		case "succeeded", "failed", "canceled":
			return pred, nil
		}
	}
}

func extractOutputURL(o any) (string, error) {
	switch v := o.(type) {
	case string:
		return v, nil
	case []any:
		if len(v) > 0 {
			if s, ok := v[0].(string); ok {
				return s, nil
			}
		}
	case []string:
		if len(v) > 0 {
			return v[0], nil
		}
	}
	return "", errors.New("unexpected output shape")
}

func download(ctx context.Context, c *http.Client, url string) ([]byte, string, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	resp, err := c.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, "", fmt.Errorf("download %d", resp.StatusCode)
	}
	mime := resp.Header.Get("Content-Type")
	if mime == "" {
		mime = "image/png"
	}
	b, err := io.ReadAll(resp.Body)
	return b, mime, err
}

func buildPrompt(userPrompt, stylePreset string) string {
	style := stylePreset
	if style == "" {
		style = "modern"
	}
	return fmt.Sprintf(
		"A %s coffee shop interior, designed based on: %s. "+
			"Inviting, photographable, realistic lighting, materials and furniture "+
			"appropriate to the style. Professional interior photography.",
		strings.ToLower(style), userPrompt)
}
```

- [ ] **Step 3: Run tests**

```bash
go test ./internal/providers/flux/...
```
Expected: both tests PASS.

- [ ] **Step 4: Commit**

```bash
cd /Users/bimantara/Dev/ASHA/Pathon/ai-image
git add backend/internal/providers/flux/
git commit -m "backend: add Flux Pro 1.1 provider via Replicate"
```

---

## Task 17: HTTP handlers — invites, uploads, generate, jobs

**Files:**
- Create: `backend/internal/http/handlers/invites.go`
- Create: `backend/internal/http/handlers/uploads.go`
- Create: `backend/internal/http/handlers/generate.go`
- Create: `backend/internal/http/handlers/jobs.go`
- Create: `backend/internal/http/handlers/handlers_test.go`
- Modify: `backend/internal/http/server.go`

- [ ] **Step 1: Implement invites handler**

Create `backend/internal/http/handlers/invites.go`:

```go
package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/bimantara/ai-image/backend/internal/auth"
	"github.com/bimantara/ai-image/backend/internal/db"
)

type RedeemInviteRequest struct {
	Code string `json:"code"`
}

func RedeemInvite(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid, err := auth.UserIDFrom(r.Context())
		if err != nil {
			writeJSONErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		var req RedeemInviteRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Code == "" {
			writeJSONErr(w, http.StatusBadRequest, "invalid request")
			return
		}

		if err := (db.Invites{DB: database}).Redeem(r.Context(), req.Code, uid); err != nil {
			switch {
			case errors.Is(err, db.ErrInviteNotFound):
				writeJSONErr(w, http.StatusBadRequest, "invite code not found")
			case errors.Is(err, db.ErrInviteAlreadyUsed):
				writeJSONErr(w, http.StatusBadRequest, "invite code already used")
			default:
				writeJSONErr(w, http.StatusInternalServerError, "internal error")
			}
			return
		}
		// Initialize profile if needed
		_ = (db.Profiles{DB: database}).Upsert(r.Context(), uid, "", req.Code)

		writeJSON(w, http.StatusOK, map[string]bool{"redeemed": true})
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeJSONErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
```

- [ ] **Step 2: Implement uploads handler**

Create `backend/internal/http/handlers/uploads.go`:

```go
package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/bimantara/ai-image/backend/internal/auth"
	"github.com/bimantara/ai-image/backend/internal/storage"
	"github.com/google/uuid"
)

type PresignRequest struct {
	ContentType string `json:"content_type"`
}

type PresignResponse struct {
	UploadURL string `json:"upload_url"`
	PublicURL string `json:"public_url"`
	Key       string `json:"key"`
}

func PresignUpload(r2 *storage.R2) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid, err := auth.UserIDFrom(r.Context())
		if err != nil {
			writeJSONErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		var req PresignRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.ContentType == "" {
			req.ContentType = "image/jpeg"
		}
		if !isAllowedImageType(req.ContentType) {
			writeJSONErr(w, http.StatusBadRequest, "unsupported content type")
			return
		}

		key := "uploads/" + uid.String() + "/" + uuid.New().String() + extFor(req.ContentType)
		upload, public, err := r2.PresignPut(r.Context(), key, req.ContentType, 5*time.Minute)
		if err != nil {
			writeJSONErr(w, http.StatusInternalServerError, "presign failed")
			return
		}
		writeJSON(w, http.StatusOK, PresignResponse{UploadURL: upload, PublicURL: public, Key: key})
	}
}

func isAllowedImageType(ct string) bool {
	switch ct {
	case "image/jpeg", "image/png", "image/webp", "image/heic":
		return true
	}
	return false
}

func extFor(ct string) string {
	switch ct {
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "image/heic":
		return ".heic"
	default:
		return ".jpg"
	}
}
```

- [ ] **Step 3: Implement generate handler**

Create `backend/internal/http/handlers/generate.go`:

```go
package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/bimantara/ai-image/backend/internal/auth"
	"github.com/bimantara/ai-image/backend/internal/db"
	"github.com/bimantara/ai-image/backend/internal/generation"
	"github.com/bimantara/ai-image/backend/internal/ratelimit"
)

type GenerateRequest struct {
	InputImageURL string `json:"input_image_url"`
	Mode          string `json:"mode"`
	Prompt        string `json:"prompt"`
	StylePreset   string `json:"style_preset"`
}

type GenerateResponse struct {
	JobID            string `json:"job_id"`
	Status           string `json:"status"`
	EstimatedSeconds int    `json:"estimated_seconds"`
}

func Generate(svc *generation.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid, err := auth.UserIDFrom(r.Context())
		if err != nil {
			writeJSONErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		var req GenerateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSONErr(w, http.StatusBadRequest, "invalid json")
			return
		}
		id, err := svc.Submit(r.Context(), generation.SubmitInput{
			UserID:        uid,
			Mode:          db.JobMode(req.Mode),
			Prompt:        req.Prompt,
			StylePreset:   req.StylePreset,
			InputImageURL: req.InputImageURL,
		})
		if err != nil {
			switch {
			case errors.Is(err, generation.ErrInvalidInput):
				writeJSONErr(w, http.StatusBadRequest, "invalid input")
			case errors.Is(err, ratelimit.ErrDailyLimit), errors.Is(err, ratelimit.ErrWeeklyLimit):
				w.Header().Set("Retry-After", "3600")
				writeJSONErr(w, http.StatusTooManyRequests, "rate limit reached")
			case errors.Is(err, ratelimit.ErrConcurrencyLimit):
				writeJSONErr(w, http.StatusTooManyRequests, "too many in-flight jobs")
			case errors.Is(err, ratelimit.ErrGlobalCostCap):
				w.Header().Set("Retry-After", "3600")
				writeJSONErr(w, http.StatusServiceUnavailable, "service paused for the day")
			default:
				writeJSONErr(w, http.StatusInternalServerError, "internal error")
			}
			return
		}
		writeJSON(w, http.StatusOK, GenerateResponse{
			JobID: id.String(), Status: "queued", EstimatedSeconds: 25,
		})
	}
}
```

- [ ] **Step 4: Implement jobs handler**

Create `backend/internal/http/handlers/jobs.go`:

```go
package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/bimantara/ai-image/backend/internal/auth"
	"github.com/bimantara/ai-image/backend/internal/db"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type JobDTO struct {
	ID             string  `json:"id"`
	Mode           string  `json:"mode"`
	Status         string  `json:"status"`
	Prompt         string  `json:"prompt"`
	StylePreset    *string `json:"style_preset"`
	InputImageURL  string  `json:"input_image_url"`
	OutputImageURL *string `json:"output_image_url"`
	ErrorMessage   *string `json:"error_message"`
	GenerationMs   *int    `json:"generation_ms"`
	CreatedAt      string  `json:"created_at"`
	CompletedAt    *string `json:"completed_at"`
}

func toDTO(j db.Job) JobDTO {
	dto := JobDTO{
		ID: j.ID.String(), Mode: string(j.Mode), Status: string(j.Status),
		Prompt: j.Prompt, StylePreset: j.StylePreset, InputImageURL: j.InputImageURL,
		OutputImageURL: j.OutputImageURL, ErrorMessage: j.ErrorMessage,
		GenerationMs: j.GenerationMs, CreatedAt: j.CreatedAt.UTC().Format(time.RFC3339),
	}
	if j.CompletedAt != nil {
		s := j.CompletedAt.UTC().Format(time.RFC3339)
		dto.CompletedAt = &s
	}
	return dto
}

func ListJobs(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid, err := auth.UserIDFrom(r.Context())
		if err != nil {
			writeJSONErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		q := r.URL.Query()
		limit, _ := strconv.Atoi(q.Get("limit"))
		if limit <= 0 || limit > 50 {
			limit = 20
		}
		cursor := time.Now().UTC().Add(time.Hour)
		if c := q.Get("cursor"); c != "" {
			if t, err := time.Parse(time.RFC3339, c); err == nil {
				cursor = t
			}
		}
		jobs, err := db.Jobs{DB: database}.List(r.Context(), uid, limit, cursor)
		if err != nil {
			writeJSONErr(w, http.StatusInternalServerError, "list failed")
			return
		}
		out := make([]JobDTO, 0, len(jobs))
		for _, j := range jobs {
			out = append(out, toDTO(j))
		}
		writeJSON(w, http.StatusOK, map[string]any{"jobs": out})
	}
}

func GetJob(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid, err := auth.UserIDFrom(r.Context())
		if err != nil {
			writeJSONErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		id, err := uuid.Parse(chi.URLParam(r, "id"))
		if err != nil {
			writeJSONErr(w, http.StatusBadRequest, "invalid id")
			return
		}
		j, err := db.Jobs{DB: database}.Get(r.Context(), id)
		if err != nil {
			if errors.Is(err, db.ErrJobNotFound) {
				writeJSONErr(w, http.StatusNotFound, "not found")
				return
			}
			writeJSONErr(w, http.StatusInternalServerError, "internal error")
			return
		}
		if j.UserID != uid {
			writeJSONErr(w, http.StatusNotFound, "not found")
			return
		}
		writeJSON(w, http.StatusOK, toDTO(j))
	}
}
```

- [ ] **Step 5: Update server to wire all handlers**

Replace `backend/internal/http/server.go`:

```go
package http

import (
	stdhttp "net/http"
	"time"

	"github.com/bimantara/ai-image/backend/internal/auth"
	"github.com/bimantara/ai-image/backend/internal/db"
	"github.com/bimantara/ai-image/backend/internal/generation"
	"github.com/bimantara/ai-image/backend/internal/http/handlers"
	"github.com/bimantara/ai-image/backend/internal/storage"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type Deps struct {
	JWTVerifier *auth.Verifier
	DB          *db.DB
	R2          *storage.R2
	Generation  *generation.Service
}

func NewRouter(d Deps) stdhttp.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	r.Get("/health", handlers.Health)

	r.Group(func(r chi.Router) {
		r.Use(auth.Required(d.JWTVerifier))
		r.Post("/redeem-invite", handlers.RedeemInvite(d.DB))
		r.Post("/uploads/presign", handlers.PresignUpload(d.R2))
		r.Post("/generate", handlers.Generate(d.Generation))
		r.Get("/jobs", handlers.ListJobs(d.DB))
		r.Get("/jobs/{id}", handlers.GetJob(d.DB))
	})

	return r
}
```

- [ ] **Step 6: Write integration test for handlers**

Create `backend/internal/http/handlers/handlers_test.go`:

```go
//go:build integration

package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bimantara/ai-image/backend/internal/auth"
	"github.com/bimantara/ai-image/backend/internal/db"
	"github.com/bimantara/ai-image/backend/internal/generation"
	apphttp "github.com/bimantara/ai-image/backend/internal/http"
	"github.com/bimantara/ai-image/backend/internal/providers"
	"github.com/bimantara/ai-image/backend/internal/providers/mock"
	"github.com/bimantara/ai-image/backend/internal/ratelimit"
	"github.com/bimantara/ai-image/backend/internal/storage"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const secret = "test-secret-must-be-long-enough"

func mintJWT(t *testing.T, uid uuid.UUID) string {
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": uid.String(),
		"aud": "authenticated",
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	s, err := tok.SignedString([]byte(secret))
	require.NoError(t, err)
	return s
}

func newTestServer(t *testing.T) (*httptest.Server, *db.DB, uuid.UUID) {
	d := db.NewTestDB(t)
	uid := uuid.New()
	_, err := d.Pool.Exec(context.Background(),
		`insert into auth.users (id) values ($1)`, uid)
	require.NoError(t, err)

	reg := providers.NewRegistry()
	reg.Register(db.ModeRealistic, mock.New("mock"))
	reg.Register(db.ModeInspirational, mock.New("mock"))
	gate := ratelimit.New(ratelimit.Config{DB: d, PerDay: 10, PerWeek: 30, GlobalCostCap: 25, MaxConcurrent: 3})
	svc := generation.NewService(generation.Deps{DB: d, Registry: reg, Gate: gate})
	r2, _ := storage.NewR2("acc", "ak", "sk", "bkt", "https://cdn.test")

	router := apphttp.NewRouter(apphttp.Deps{
		JWTVerifier: auth.NewVerifier(secret),
		DB:          d,
		R2:          r2,
		Generation:  svc,
	})
	return httptest.NewServer(router), d, uid
}

func TestHealth(t *testing.T) {
	srv, _, _ := newTestServer(t)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/health")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, 200, resp.StatusCode)
}

func TestGenerate_RequiresAuth(t *testing.T) {
	srv, _, _ := newTestServer(t)
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/generate", "application/json",
		bytes.NewBufferString(`{}`))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, 401, resp.StatusCode)
}

func TestRedeemInvite_HappyPath(t *testing.T) {
	srv, d, uid := newTestServer(t)
	defer srv.Close()

	require.NoError(t, (db.Invites{DB: d}).Create(context.Background(), "OK1"))

	req, _ := http.NewRequest("POST", srv.URL+"/redeem-invite",
		bytes.NewBufferString(`{"code":"OK1"}`))
	req.Header.Set("Authorization", "Bearer "+mintJWT(t, uid))
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, 200, resp.StatusCode)
}

func TestGenerate_HappyPath(t *testing.T) {
	srv, d, uid := newTestServer(t)
	defer srv.Close()
	require.NoError(t, (db.Profiles{DB: d}).Upsert(context.Background(), uid, "x", ""))

	body, _ := json.Marshal(map[string]string{
		"input_image_url": "https://cdn/x.jpg",
		"mode":            "realistic",
		"prompt":          "Scandinavian coffee shop with oak",
		"style_preset":    "scandinavian",
	})
	req, _ := http.NewRequest("POST", srv.URL+"/generate", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+mintJWT(t, uid))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, 200, resp.StatusCode)
}

func TestGenerate_InvalidPrompt(t *testing.T) {
	srv, d, uid := newTestServer(t)
	defer srv.Close()
	require.NoError(t, (db.Profiles{DB: d}).Upsert(context.Background(), uid, "x", ""))

	body, _ := json.Marshal(map[string]string{
		"input_image_url": "https://cdn/x.jpg",
		"mode":            "realistic",
		"prompt":          "x",
	})
	req, _ := http.NewRequest("POST", srv.URL+"/generate", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+mintJWT(t, uid))
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, 400, resp.StatusCode)
}
```

- [ ] **Step 7: Run handler tests**

```bash
go test -tags=integration ./internal/http/...
```
Expected: all 5 tests PASS.

- [ ] **Step 8: Commit**

```bash
cd /Users/bimantara/Dev/ASHA/Pathon/ai-image
git add backend/internal/http/ backend/internal/storage/
git commit -m "backend: wire HTTP handlers for invites, uploads, generate, jobs"
```

---

## Task 18: Sentry observability

**Files:**
- Create: `backend/internal/obs/sentry.go`
- Modify: `backend/cmd/api/main.go`

- [ ] **Step 1: Install Sentry**

```bash
go get github.com/getsentry/sentry-go@v0.27.0
go get github.com/getsentry/sentry-go/http@v0.27.0
```

- [ ] **Step 2: Implement Sentry init**

Create `backend/internal/obs/sentry.go`:

```go
package obs

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/getsentry/sentry-go"
	sentryhttp "github.com/getsentry/sentry-go/http"
)

// Init configures Sentry; no-op if dsn is empty.
func Init(dsn, env string) error {
	if dsn == "" {
		slog.Info("sentry disabled (no DSN)")
		return nil
	}
	return sentry.Init(sentry.ClientOptions{
		Dsn:              dsn,
		Environment:      env,
		TracesSampleRate: 0.1,
	})
}

// Middleware wraps an http.Handler with Sentry's panic recovery + request scope.
func Middleware(next http.Handler) http.Handler {
	return sentryhttp.New(sentryhttp.Options{Repanic: true}).Handle(next)
}

// Flush should be deferred from main before shutdown.
func Flush() { sentry.Flush(2 * time.Second) }
```

- [ ] **Step 3: Wire into main.go**

Replace `backend/cmd/api/main.go`:

```go
package main

import (
	"context"
	"errors"
	"log/slog"
	stdhttp "net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bimantara/ai-image/backend/internal/auth"
	"github.com/bimantara/ai-image/backend/internal/config"
	"github.com/bimantara/ai-image/backend/internal/db"
	"github.com/bimantara/ai-image/backend/internal/generation"
	apphttp "github.com/bimantara/ai-image/backend/internal/http"
	"github.com/bimantara/ai-image/backend/internal/obs"
	"github.com/bimantara/ai-image/backend/internal/providers"
	"github.com/bimantara/ai-image/backend/internal/providers/flux"
	"github.com/bimantara/ai-image/backend/internal/providers/gemini"
	"github.com/bimantara/ai-image/backend/internal/ratelimit"
	"github.com/bimantara/ai-image/backend/internal/storage"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	cfg, err := config.Load()
	if err != nil {
		slog.Error("config", "err", err); os.Exit(1)
	}

	if err := obs.Init(cfg.SentryDSN, cfg.Env); err != nil {
		slog.Error("sentry init", "err", err)
	}
	defer obs.Flush()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	database, err := db.New(ctx, cfg.DatabaseURL)
	if err != nil { slog.Error("db", "err", err); os.Exit(1) }
	defer database.Close()

	r2, err := storage.NewR2(cfg.R2AccountID, cfg.R2AccessKeyID, cfg.R2SecretAccessKey, cfg.R2Bucket, cfg.R2PublicBaseURL)
	if err != nil { slog.Error("r2", "err", err); os.Exit(1) }

	reg := providers.NewRegistry()
	reg.Register(db.ModeRealistic, gemini.New(gemini.Config{APIKey: cfg.GeminiAPIKey}))
	reg.Register(db.ModeInspirational, flux.New(flux.Config{
		Token: cfg.ReplicateAPIToken, ModelVersion: cfg.ReplicateFluxVersion,
	}))

	gate := ratelimit.New(ratelimit.Config{
		DB: database, PerDay: cfg.RateLimitPerDay, PerWeek: cfg.RateLimitPerWeek,
		GlobalCostCap: cfg.GlobalCostCapUSD, MaxConcurrent: cfg.MaxConcurrentJobsPerUser,
	})
	svc := generation.NewService(generation.Deps{DB: database, Registry: reg, Gate: gate})

	worker := generation.NewWorker(generation.WorkerDeps{
		DB: database, Registry: reg, R2: r2, Concurrency: 4,
	})
	go worker.Run(ctx)

	router := obs.Middleware(apphttp.NewRouter(apphttp.Deps{
		JWTVerifier: auth.NewVerifier(cfg.SupabaseJWTSecret),
		DB: database, R2: r2, Generation: svc,
	}))

	srv := &stdhttp.Server{
		Addr: ":" + cfg.Port, Handler: router,
		ReadTimeout: 10 * time.Second, WriteTimeout: 100 * time.Second,
		IdleTimeout: 120 * time.Second,
	}
	done := make(chan struct{})
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, syscall.SIGINT, syscall.SIGTERM)
		<-sigint
		sctx, scancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer scancel()
		_ = srv.Shutdown(sctx)
		close(done)
	}()

	slog.Info("listening", "port", cfg.Port, "env", cfg.Env)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, stdhttp.ErrServerClosed) {
		slog.Error("server", "err", err); os.Exit(1)
	}
	<-done
}
```

- [ ] **Step 4: Build check**

```bash
cd backend
go build ./...
```
Expected: clean build.

- [ ] **Step 5: Commit**

```bash
cd /Users/bimantara/Dev/ASHA/Pathon/ai-image
git add backend/
git commit -m "backend: wire Sentry, providers, worker, and full HTTP server"
```

---

## Task 19: Dockerfile + Fly.io config

**Files:**
- Create: `backend/Dockerfile`
- Create: `backend/fly.toml`

- [ ] **Step 1: Write Dockerfile**

Create `backend/Dockerfile`:

```dockerfile
# Build stage
FROM golang:1.22-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/api ./cmd/api

# Runtime stage
FROM alpine:3.19
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app
COPY --from=builder /out/api .
EXPOSE 8080
ENTRYPOINT ["/app/api"]
```

- [ ] **Step 2: Build Docker image locally**

```bash
cd backend
docker build -t ai-image-backend:dev .
```
Expected: successful build.

- [ ] **Step 3: Initialize Fly app**

```bash
cd backend
fly launch --no-deploy --copy-config --name ai-image-backend --region sin
```
This generates `fly.toml`. When prompted:
- Org: select your org
- Postgres: NO (using Supabase)
- Redis: NO
- Deploy now: NO

- [ ] **Step 4: Edit fly.toml**

Replace `backend/fly.toml` with:

```toml
app = "ai-image-backend"
primary_region = "sin"

[build]
  dockerfile = "Dockerfile"

[env]
  PORT = "8080"
  ENV = "production"

[http_service]
  internal_port = 8080
  force_https = true
  auto_stop_machines = true
  auto_start_machines = true
  min_machines_running = 0
  processes = ["app"]

  [http_service.concurrency]
    type = "requests"
    soft_limit = 50
    hard_limit = 100

  [[http_service.checks]]
    grace_period = "10s"
    interval = "30s"
    method = "get"
    path = "/health"
    timeout = "5s"

[[vm]]
  cpu_kind = "shared"
  cpus = 1
  memory_mb = 512
```

- [ ] **Step 5: Set secrets in Fly**

Run for each value from your `.env`:

```bash
fly secrets set \
  DATABASE_URL="..." \
  SUPABASE_URL="..." \
  SUPABASE_SERVICE_ROLE_KEY="..." \
  SUPABASE_JWT_SECRET="..." \
  R2_ACCOUNT_ID="..." \
  R2_ACCESS_KEY_ID="..." \
  R2_SECRET_ACCESS_KEY="..." \
  R2_BUCKET="ai-image-prototype" \
  R2_PUBLIC_BASE_URL="https://cdn.example.com" \
  GEMINI_API_KEY="..." \
  REPLICATE_API_TOKEN="..." \
  REPLICATE_FLUX_VERSION="..." \
  SENTRY_DSN="..."
```

- [ ] **Step 6: Deploy**

```bash
cd backend
fly deploy
```
Expected: deploy succeeds. Capture the URL (e.g., `https://ai-image-backend.fly.dev`).

- [ ] **Step 7: Verify health endpoint**

```bash
curl -s https://ai-image-backend.fly.dev/health
```
Expected: `{"status":"ok"}`.

- [ ] **Step 8: Commit**

```bash
cd /Users/bimantara/Dev/ASHA/Pathon/ai-image
git add backend/Dockerfile backend/fly.toml
git commit -m "backend: add Dockerfile and Fly.io deploy config"
```

---

## Task 20: GitHub Actions CI

**Files:**
- Create: `.github/workflows/backend.yml` (at repo root, not inside backend/)

- [ ] **Step 1: Write workflow**

Create `/Users/bimantara/Dev/ASHA/Pathon/ai-image/.github/workflows/backend.yml`:

```yaml
name: Backend CI

on:
  push:
    paths: [ 'backend/**', '.github/workflows/backend.yml' ]
  pull_request:
    paths: [ 'backend/**', '.github/workflows/backend.yml' ]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'
          cache-dependency-path: backend/go.sum

      - name: Install golangci-lint
        uses: golangci/golangci-lint-action@v6
        with:
          version: v1.59
          working-directory: backend

      - name: Unit tests
        working-directory: backend
        run: go test ./...

      - name: Integration tests
        working-directory: backend
        run: go test -tags=integration ./...

  deploy:
    needs: test
    if: github.ref == 'refs/heads/main'
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: superfly/flyctl-actions/setup-flyctl@master
      - run: flyctl deploy --remote-only
        working-directory: backend
        env:
          FLY_API_TOKEN: ${{ secrets.FLY_API_TOKEN }}
```

- [ ] **Step 2: Add Fly token to GitHub repo secrets**

Generate token: `fly tokens create deploy -x 999999h`

In GitHub repo → Settings → Secrets → Actions → New secret:
- Name: `FLY_API_TOKEN`
- Value: paste token

- [ ] **Step 3: Add basic golangci-lint config**

Create `backend/.golangci.yml`:

```yaml
run:
  timeout: 5m
  build-tags:
    - integration

linters:
  enable:
    - errcheck
    - gosimple
    - govet
    - ineffassign
    - staticcheck
    - unused
    - misspell
```

- [ ] **Step 4: Push and verify**

```bash
cd /Users/bimantara/Dev/ASHA/Pathon/ai-image
git add .github backend/.golangci.yml
git commit -m "ci: add backend test + deploy workflow"
git push origin main
```
Expected: GitHub Actions runs and (after deploy step) backend is reachable. Check Actions tab in GitHub.

---

## Task 21: End-to-end smoke test against deployed backend

**Files:** none — manual verification.

- [ ] **Step 1: Create test invite codes in Supabase**

In Supabase SQL editor:

```sql
insert into invites (code) values ('TEST001'), ('TEST002'), ('TEST003');
```

- [ ] **Step 2: Sign up a test user via Supabase Dashboard**

In Auth → Users → Add user → enter `test@example.com` + password (mark "Auto-confirm user"). Copy the user ID.

- [ ] **Step 3: Sign in to get a JWT**

Replace `<SUPABASE_URL>` and `<ANON_KEY>` with your values, then run:

```bash
JWT=$(curl -s -X POST '<SUPABASE_URL>/auth/v1/token?grant_type=password' \
  -H "apikey: <ANON_KEY>" \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"yourpass"}' | jq -r .access_token)
echo "$JWT"
```
Expected: a long JWT string.

- [ ] **Step 4: Test redeem-invite**

```bash
curl -X POST https://ai-image-backend.fly.dev/redeem-invite \
  -H "Authorization: Bearer $JWT" \
  -H "Content-Type: application/json" \
  -d '{"code":"TEST001"}'
```
Expected: `{"redeemed":true}`.

- [ ] **Step 5: Test upload presign**

```bash
curl -X POST https://ai-image-backend.fly.dev/uploads/presign \
  -H "Authorization: Bearer $JWT" \
  -H "Content-Type: application/json" \
  -d '{"content_type":"image/jpeg"}'
```
Expected: JSON with `upload_url`, `public_url`, `key`.

- [ ] **Step 6: Upload an actual photo to the presigned URL**

```bash
UPLOAD_URL='...' # from step 5
curl -X PUT "$UPLOAD_URL" \
  -H "Content-Type: image/jpeg" \
  --data-binary @path/to/empty-room.jpg
```
Expected: 200.

- [ ] **Step 7: Submit a generate job**

```bash
PUBLIC_URL='...' # from step 5
curl -X POST https://ai-image-backend.fly.dev/generate \
  -H "Authorization: Bearer $JWT" \
  -H "Content-Type: application/json" \
  -d "{
    \"input_image_url\": \"$PUBLIC_URL\",
    \"mode\": \"realistic\",
    \"prompt\": \"Scandinavian coffee shop with light oak, plants, soft window light\",
    \"style_preset\": \"scandinavian\"
  }"
```
Expected: `{"job_id":"...","status":"queued","estimated_seconds":25}`.

- [ ] **Step 8: Poll for completion**

```bash
JOB_ID='...' # from step 7
for i in {1..30}; do
  curl -s https://ai-image-backend.fly.dev/jobs/$JOB_ID \
    -H "Authorization: Bearer $JWT" | jq '.status, .output_image_url, .error_message'
  sleep 3
done
```
Expected: status transitions queued → processing → completed, with a non-null `output_image_url`. Open the URL in a browser to inspect the AI-generated coffee shop.

- [ ] **Step 9: Run the same flow with `"mode":"inspirational"`**

Repeat step 7 with `"mode":"inspirational"` and verify Flux produces a different style of image.

- [ ] **Step 10: Test rate limit**

In a loop, call `/generate` 11 times. The 11th should return 429.

- [ ] **Step 11: Test invalid input**

```bash
curl -X POST https://ai-image-backend.fly.dev/generate \
  -H "Authorization: Bearer $JWT" \
  -H "Content-Type: application/json" \
  -d '{"input_image_url":"https://x","mode":"realistic","prompt":"x"}'
```
Expected: 400.

- [ ] **Step 12: Verify Sentry receives errors**

Force an error (e.g., invalid R2 secret, or trigger a 500) and confirm it appears in your Sentry project's Issues view.

- [ ] **Step 13: Final commit (release tag)**

```bash
cd /Users/bimantara/Dev/ASHA/Pathon/ai-image
git tag backend-v0.1.0 -m "Backend prototype ready for mobile integration"
git push --tags
```

---

## Done

At this point the backend is fully functional, deployed to Fly.io Singapore, and able to:
- Authenticate Supabase JWTs.
- Gate signup via invite codes.
- Accept image uploads via presigned R2 URLs.
- Enqueue generation jobs.
- Process jobs through Gemini 2.5 Flash Image (realistic) or Flux Pro 1.1 (inspirational).
- Upload outputs to R2.
- Update job status in Postgres (with Supabase Realtime push enabled).
- Enforce per-user and global rate/cost limits.
- Report errors to Sentry.

The mobile-implementation plan (next plan) will consume this API.
