# Coffee Shop Design Generator — Prototype Design Spec

**Date**: 2026-05-21
**Status**: Approved for prototype implementation
**Author**: Brainstormed with Claude

## Overview

A mobile app that lets a user photograph an empty room, describe the coffee shop they want it to become, and receive an AI-generated visualization of the designed space. Two generation modes: **Realistic** (preserve original room structure) and **Inspirational** (free reinterpretation).

This spec covers the **closed-beta prototype** — used to validate product hypothesis with ~50 invited users before investing in a production-grade rebuild.

## Goals

- Validate that the dual-mode generation (Realistic + Inspirational) produces output good enough that target users (coffee shop owners / aspiring owners) find it valuable.
- Get end-to-end flow working on real iOS + Android devices, so the team can experience the full UX, not just AI quality.
- Build with ~80% code reuse for the future mature version — no throwaway architecture for components that survive the rebuild.
- Ship in ~2 weeks of solo dev work.

## Non-goals (explicit cuts vs. mature version)

- **No subscription, paywall, or RevenueCat integration** — beta is free for invited users.
- **No App Store / Play Store public release** — distribute via TestFlight (iOS) and Google Play Internal Testing track.
- **No public signup** — invite-only via codes.
- **No multi-region deploy** — single region (Singapore) is enough for beta.
- **No production observability stack** — Sentry for errors is enough; skip Grafana/metrics dashboards.
- **No localization** — English only for prototype. (Mature version: Indonesian + English.)

## Target Users (prototype)

~50 invited beta users — mix of:
- Current coffee shop owners considering renovation.
- Aspiring owners scoping out a space.
- Interior designers curious about AI tools.

Recruited manually via personal network, WhatsApp, and a single Twitter/IG post.

## Core User Flows

### Flow 1: Onboarding
1. User opens app → welcome screen.
2. Taps "Enter invite code" → enters code received from team.
3. Backend validates code → marks as used.
4. User signs up via email / Apple / Google (Supabase Auth). **Sign in with Apple is mandatory** for iOS builds that ship any third-party social login (Apple guideline).
5. Lands on home screen with brief tutorial overlay.

### Flow 2: Generate
1. From home, user taps "New Design".
2. Picks a photo (camera or gallery) — basic crop available.
3. Selects mode: **Realistic** or **Inspirational** (with one-line explanation of each).
4. (Optional) Picks a style preset chip: Scandinavian, Industrial, Japandi, Cozy Warm, Minimalist, or "None".
5. Types description prompt (5–500 chars).
6. Taps "Generate".
7. App shows progress UI (queued → processing → ~10–30s).
8. Result appears with the original photo for before/after comparison.
9. User can: save to device, share, regenerate with tweaked prompt.

### Flow 3: Gallery
1. Bottom-nav "Gallery" → grid of past generations, newest first.
2. Tap → detail view with original + result, prompt used, regenerate button.
3. Pull to refresh, infinite scroll.

### Flow 4: Rate Limit Hit
1. User has used 10 generates today → tap Generate → toast: "Daily limit reached. Resets at midnight UTC."
2. Compose form disabled with same message inline.

## Architecture

```
┌─────────────────┐         ┌──────────────────────┐         ┌─────────────────────┐
│  Flutter App    │ HTTPS   │   Go Backend         │         │  AI Providers       │
│  (iOS/Android)  │ ──────► │   Fly.io Singapore   │ ──────► │  • Gemini 2.5 Flash │
│                 │         │                      │         │    Image (realistic)│
│  • Auth         │ ◄────── │  • JWT verify        │ ◄────── │  • Flux Pro 1.1     │
│  • Capture      │   JSON  │  • Rate limit        │         │    via Replicate    │
│  • Compose      │         │  • Provider routing  │         │    (inspirational)  │
│  • Realtime     │         │  • Cost guard        │         └─────────────────────┘
│  • Gallery      │         │  • Job worker pool   │
└─────────────────┘         └──────────┬───────────┘         ┌─────────────────────┐
                                       │                     │  Cloudflare R2      │
                                       │             ──────► │  (S3-compatible,    │
                                       │                     │   free egress,      │
                                       │                     │   global CDN)       │
                                       ▼                     └─────────────────────┘
                            ┌──────────────────────┐
                            │  Supabase            │
                            │  • Postgres (jobs,   │
                            │    profiles, invites)│
                            │  • Auth              │
                            │  • Realtime (job     │
                            │    status push)      │
                            └──────────────────────┘
```

**Why Fly.io Singapore**: low latency to Indonesia (~10ms), reasonable to other Asian beta users, auto-scale to zero saves money during low usage, easy multi-region expansion later.

**Why R2 over Supabase Storage**: free egress means image delivery doesn't surprise the cost ceiling; built-in CDN keeps results fast globally.

**Why Supabase**: bundles Postgres + Auth + Realtime, generous free tier covers entire beta. Realtime is critical for pushing job status updates without polling.

## Tech Stack

### Mobile (`mobile/`)
- **Framework**: Flutter (Dart)
- **Routing**: `go_router`
- **State**: `riverpod` (chosen for codegen + compile-time safety; revisit if team prefers `bloc`)
- **HTTP**: `dio` with interceptor for JWT injection
- **Auth & DB**: `supabase_flutter`
- **Image**: `image_picker`, `image_cropper`, `cached_network_image`, `flutter_image_compress`
- **Realtime**: Supabase Realtime channel subscription
- **Local storage**: `flutter_secure_storage` (JWT), `shared_preferences` (settings)
- **Error tracking**: `sentry_flutter`

### Backend (`backend/`)
- **Language**: Go 1.22+
- **Router**: `chi`
- **Postgres driver**: `pgx/v5`
- **Auth**: verify Supabase JWT using `golang-jwt/jwt/v5`
- **S3 client**: `aws-sdk-go-v2` (pointed at R2 endpoint)
- **AI providers**: raw `net/http` clients (no official Go SDKs needed for Gemini or Replicate)
- **Background jobs**: in-process goroutine worker pool (no external queue for MVP)
- **Error tracking**: `getsentry/sentry-go`
- **Config**: env vars via `kelseyhightower/envconfig`

### Infrastructure
- **Backend hosting**: Fly.io (single region: `sin`)
- **Database & Auth**: Supabase (free tier, region: Singapore)
- **Object storage**: Cloudflare R2 + Cloudflare CDN
- **AI providers**:
  - Google AI Studio (Gemini 2.5 Flash Image)
  - Replicate (Flux Pro 1.1)
- **CI/CD**: GitHub Actions
- **Mobile distribution**: TestFlight (iOS) + Google Play Internal Testing (Android)

## Database Schema

```sql
-- managed by Supabase Auth
-- auth.users (id, email, ...)

create table invites (
  code text primary key,
  used_by uuid references auth.users(id),
  created_at timestamptz not null default now(),
  used_at timestamptz
);

create table profiles (
  user_id uuid primary key references auth.users(id) on delete cascade,
  display_name text,
  invite_code text references invites(code),
  generates_today int not null default 0,
  generates_week int not null default 0,
  reset_at timestamptz not null default now(),
  created_at timestamptz not null default now()
);

create type job_status as enum ('queued', 'processing', 'completed', 'failed');
create type job_mode as enum ('realistic', 'inspirational');

create table jobs (
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

create index jobs_user_created_idx on jobs (user_id, created_at desc);
create index jobs_status_idx on jobs (status) where status in ('queued', 'processing');

create table daily_costs (
  date date primary key,
  total_cost_usd numeric(10, 4) not null default 0,
  generate_count int not null default 0
);
```

**Row-Level Security** (Supabase RLS):
- `profiles`: user can only read/update own row.
- `jobs`: user can only read own jobs. Insert disabled from client (only backend service role can insert).
- `invites`, `daily_costs`: client has no access.

## API Surface (Backend)

All routes JWT-authenticated except `/health` and `/redeem-invite`.

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/health` | liveness probe |
| `POST` | `/redeem-invite` | mark invite code used, requires JWT |
| `POST` | `/uploads/presign` | return signed PUT URL for R2 upload of input photo |
| `POST` | `/generate` | enqueue a generation job, return `job_id` |
| `GET` | `/jobs` | list user's jobs (paginated, cursor) |
| `GET` | `/jobs/:id` | get one job's current state |

### `POST /generate` request shape
```json
{
  "input_image_url": "https://r2.../uploads/<uuid>.jpg",
  "mode": "realistic" | "inspirational",
  "prompt": "Scandinavian coffee shop with white oak ...",
  "style_preset": "scandinavian" | null
}
```

### Response
```json
{
  "job_id": "uuid",
  "status": "queued",
  "estimated_seconds": 25
}
```

Flutter then subscribes to Supabase Realtime channel `jobs:id=eq.<job_id>` for live status updates.

## AI Provider Integration

### Provider abstraction
```go
package providers

type Input struct {
    InputImageURL string
    Prompt        string
    StylePreset   string
}

type Result struct {
    OutputImageBytes  []byte
    ProviderRequestID string
    EstimatedCostUSD  float64
}

type ImageProvider interface {
    Generate(ctx context.Context, in Input) (Result, error)
    Name() string
}
```

Routing: realistic mode → `gemini.Provider`, inspirational mode → `flux.Provider`.

### Gemini 2.5 Flash Image (realistic)
- Endpoint: Google AI Studio `generateContent` with image input + text prompt.
- Strong at preserving spatial structure when given an input image and edit-style prompt.
- Cost: ~$0.039 / image. Latency: ~5–15s.
- Response: base64 image → decode → upload to R2.

### Flux Pro 1.1 via Replicate (inspirational)
- Endpoint: `https://api.replicate.com/v1/predictions` with model version pinned.
- Use input image as `image_prompt` (soft reference) + text prompt.
- Cost: ~$0.04 / image. Latency: ~10–25s.
- Replicate returns a hosted URL → download → re-upload to R2 (we own URL lifetime, serve via our CDN, no dependency on Replicate's storage).

### Prompt template (backend-side enrichment)
We don't pass raw user prompt directly. Backend wraps it:

```
A {style_preset || "modern"} coffee shop interior, designed based on:
{user_prompt}

The space should feel inviting and photographable. Include realistic
lighting, materials, and furniture appropriate to the style.
```

This boosts result quality without making the user think hard about prompting.

## Anti-Abuse (critical because beta is free)

1. **Invite-only signup**: Backend generates 50 unique codes manually, team shares them. `POST /redeem-invite` marks used; one code per user.
2. **Per-user rate limit**: 10 generates/day, 30/week. Enforced at `POST /generate` by checking `profiles.generates_today/_week` and incrementing atomically. Daily counter reset by cron at 00:00 UTC.
3. **Global cost cap**: Backend tracks `daily_costs.total_cost_usd`. If > $25/day, `POST /generate` returns 503 with message; alert email fires. Resets at 00:00 UTC.
   - Math: 50 users × 10 generates/day × $0.04 = $20 absolute max. $25 cap gives ~25% margin for occasional overage; tune down once real usage data is in.
4. **Concurrency limit**: Max 3 in-flight jobs per user. Enforced by count query before insert.

## Error Handling

| Scenario | Handling |
|---|---|
| AI provider timeout (>90s) | Worker marks job `failed`, error_message=`timeout`, rate-limit counter NOT incremented (user not charged the quota). |
| AI provider 5xx / rate limit | Retry up to 2x with exponential backoff (2s, 8s). If still failing, mark `failed`. |
| Content moderation reject | Mark `failed`, error_message includes provider's reason. Flutter shows friendly: "Your prompt was filtered. Try rewording without brand names or specific people." |
| R2 upload fail | Worker retries 3x. If still failing, mark `failed`, error_message=`storage_error`. |
| Flutter upload fail (network) | dio retry 3x with backoff; if still failing, show "Retry" button, do NOT create job. |
| Job stuck `processing` >5min | Cron sweep marks stale jobs `failed` with `timeout`. |
| Invite code invalid / already used | 400 with clear message. |
| Rate limit hit | 429 with `Retry-After` header set to seconds until reset. |
| Subscription / cost cap hit (global) | 503 with `Retry-After`. |

All backend errors logged to Sentry with `user_id`, `job_id`, `request_id`. Flutter errors also to Sentry.

## Testing Approach

### Backend (Go)
- **Unit tests**: handlers, business logic, provider mapping (mocked HTTP via `httptest.Server`), rate limit logic.
- **Integration tests**: `testcontainers-go` for real Postgres; full HTTP flow; AI providers mocked.
- **Contract tests**: fixture-based, asserting provider response mapping. Re-record fixtures monthly.

### Mobile (Flutter)
- **Widget tests**: paywall (none for MVP), compose form validation, gallery list, auth screens.
- **Integration tests** (`integration_test/`): 3 critical happy paths — invite redeem + signup, generate flow with mocked backend, gallery list + detail.

### AI Quality Eval
- Internal eval set: 20–30 paired (empty room photo + target prompt) examples.
- Run on first integration, then monthly. Score 1–5 on: prompt fidelity, photorealism, structure preservation (realistic mode), artifacts.
- Stored in shared spreadsheet.

### Manual QA before each TestFlight / Internal Testing release
- 1 real iOS device + 1 real Android device.
- 5 generates (both modes), assess visually.
- Invite redeem flow.
- Rate limit hit behavior.

### CI/CD
- **GitHub Actions** workflows:
  - `backend.yml`: `go test`, `golangci-lint`, build Docker, push to Fly registry, deploy.
  - `mobile.yml`: `flutter analyze`, `flutter test`, build APK + IPA on tag.
- Branch protection: tests must pass before merge to `main`.

## Deployment

### Backend
- Dockerfile multi-stage build (small final image).
- Fly.io app: 1 region (`sin`), 1 machine (shared-cpu-1x, 512MB), scale-to-zero enabled.
- Postgres on Supabase (free tier).
- Env vars set via `fly secrets set` (Gemini key, Replicate key, R2 creds, Supabase service role key, Sentry DSN).

### Mobile
- **iOS**: TestFlight build. Manual Xcode signing or Fastlane. **Requires Apple Developer Program** ($99/year).
- **Android**: Google Play Internal Testing track. Manual upload of signed AAB. **Requires Google Play Developer account** ($25 one-time).

### Domains
- Backend: `api.<domain>` behind Cloudflare DNS + TLS.
- R2 public bucket: `cdn.<domain>` (custom domain attached to R2 bucket).

## Estimated Effort

- Backend skeleton + auth + jobs + 2 providers + rate limit + Sentry: **3–4 days**
- Flutter skeleton + auth + capture + compose + realtime + gallery + Sentry: **4–5 days**
- Polish, manual QA, TestFlight + Internal Testing release: **2 days**
- **Total**: ~2 weeks solo

## Roadmap to Mature Version (post-validation)

Once beta validates the product, the rebuild adds:
- **Subscription via RevenueCat** — paywall, webhook handling, entitlement caching.
- **Public release** to App Store + Play Store with subscription tiers.
- **Localization** — Indonesian + English.
- **Multi-region backend** (Fly.io: SIN + IAD + FRA) for global latency.
- **Production observability** — Grafana Cloud / similar, full metrics dashboard.
- **Cost guard per user per day** (replaces beta's simple counter).
- **Subscription state reconciliation cron** (handles webhook losses).
- **Marketing site** + onboarding videos.
- **More style presets, advanced prompt enhancer, re-generate-with-tweaks history.**

**Reusable from prototype (~80%)**:
- All backend providers + abstraction.
- Job schema + worker pool (add subscription gate).
- Flutter capture / compose / gallery modules.
- R2 storage layer.
- Auth flow (Supabase).

**Discarded / rebuilt**:
- Invite system (replaced by public signup + paywall).
- Rate limit logic (replaced by subscription quota).
- Single-region deploy (replaced by multi-region).

## Open Questions (to revisit post-beta)

- Will users want to compare multiple style variants from one prompt (current: 1 generate = 1 result)?
- Should we generate from a 3D layout instead of a single photo? (More setup but better fidelity.)
- Is "Inspirational" mode actually used, or do users overwhelmingly prefer "Realistic"? (Drives whether we keep both.)
- What style presets get used most? (Drives the preset list in mature version.)
