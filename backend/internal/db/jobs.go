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
