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
