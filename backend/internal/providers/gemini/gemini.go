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
