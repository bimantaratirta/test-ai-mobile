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
