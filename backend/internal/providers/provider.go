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
