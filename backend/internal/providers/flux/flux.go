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
	defaultBaseURL        = "https://api.replicate.com/v1"
	estimatedCostUSD      = 0.04
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
			"prompt":           buildPrompt(in.Prompt, in.StylePreset),
			"image_prompt":     in.InputImageURL,
			"aspect_ratio":     "4:3",
			"output_format":    "png",
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
