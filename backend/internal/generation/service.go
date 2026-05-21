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
	jobs := db.Jobs{DB: s.d.DB}
	id, err := jobs.Insert(ctx, db.InsertJobParams{
		UserID: in.UserID, Mode: in.Mode, Prompt: in.Prompt,
		StylePreset: in.StylePreset, InputImageURL: in.InputImageURL,
	})
	if err != nil {
		return uuid.Nil, err
	}
	profiles := db.Profiles{DB: s.d.DB}
	if err := profiles.IncrementCounters(ctx, in.UserID); err != nil {
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
