package openrouter

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
	defaultBaseURL    = "https://openrouter.ai/api/v1"
	estimatedCostUSD  = 0.039 // approximate per-image cost via OpenRouter
	requestTimeoutSec = 60
)

type Config struct {
	APIKey  string
	BaseURL string
	Model   string // e.g. "google/gemini-2.5-flash-image-preview"
	Name    string // provider name reported via Name() — defaults to "openrouter"
	Client  *http.Client
}

type Provider struct{ cfg Config }

func New(cfg Config) *Provider {
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultBaseURL
	}
	if cfg.Name == "" {
		cfg.Name = "openrouter"
	}
	if cfg.Client == nil {
		cfg.Client = &http.Client{Timeout: requestTimeoutSec * time.Second}
	}
	return &Provider{cfg: cfg}
}

func (p *Provider) Name() string { return p.cfg.Name }

type chatReq struct {
	Model      string    `json:"model"`
	Messages   []message `json:"messages"`
	Modalities []string  `json:"modalities,omitempty"`
}

type message struct {
	Role    string           `json:"role"`
	Content []messageContent `json:"content"`
}

type messageContent struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *imageURL `json:"image_url,omitempty"`
}

type imageURL struct {
	URL string `json:"url"`
}

type chatResp struct {
	ID      string   `json:"id"`
	Choices []choice `json:"choices"`
	Error   *struct {
		Message string `json:"message"`
		Code    int    `json:"code"`
	} `json:"error,omitempty"`
}

type choice struct {
	Message      respMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

type respMessage struct {
	Role    string      `json:"role"`
	Content string      `json:"content"`
	Images  []respImage `json:"images,omitempty"`
}

type respImage struct {
	Type     string    `json:"type"`
	ImageURL *imageURL `json:"image_url,omitempty"`
}

func (p *Provider) Generate(ctx context.Context, in providers.Input) (providers.Result, error) {
	imgBytes, imgMime, err := fetchImage(ctx, p.cfg.Client, in.InputImageURL)
	if err != nil {
		return providers.Result{}, fmt.Errorf("fetch input image: %w", err)
	}
	dataURI := fmt.Sprintf("data:%s;base64,%s", imgMime, base64.StdEncoding.EncodeToString(imgBytes))

	prompt := buildPrompt(in.Prompt, in.StylePreset)

	body := chatReq{
		Model: p.cfg.Model,
		Messages: []message{{
			Role: "user",
			Content: []messageContent{
				{Type: "text", Text: prompt},
				{Type: "image_url", ImageURL: &imageURL{URL: dataURI}},
			},
		}},
		Modalities: []string{"image", "text"},
	}
	b, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, "POST", p.cfg.BaseURL+"/chat/completions", bytes.NewReader(b))
	if err != nil {
		return providers.Result{}, err
	}
	req.Header.Set("Authorization", "Bearer "+p.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")
	// OpenRouter recommends these for attribution / abuse tracking
	req.Header.Set("HTTP-Referer", "https://github.com/bimantara/ai-image")
	req.Header.Set("X-Title", "AI Image Coffee Shop Designer")

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
		return providers.Result{}, fmt.Errorf("openrouter %d: %s", resp.StatusCode, string(raw))
	}

	var out chatResp
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return providers.Result{}, err
	}
	if out.Error != nil {
		// OpenRouter reports content moderation via error messages sometimes
		if strings.Contains(strings.ToLower(out.Error.Message), "safety") ||
			strings.Contains(strings.ToLower(out.Error.Message), "filter") ||
			strings.Contains(strings.ToLower(out.Error.Message), "moderation") {
			return providers.Result{}, providers.ErrContentModerated
		}
		return providers.Result{}, fmt.Errorf("openrouter error: %s", out.Error.Message)
	}
	if len(out.Choices) == 0 {
		return providers.Result{}, errors.New("no choices returned")
	}
	ch := out.Choices[0]
	if ch.FinishReason == "content_filter" {
		return providers.Result{}, providers.ErrContentModerated
	}
	if len(ch.Message.Images) == 0 {
		return providers.Result{}, errors.New("no images in response")
	}
	imgRef := ch.Message.Images[0]
	if imgRef.ImageURL == nil || imgRef.ImageURL.URL == "" {
		return providers.Result{}, errors.New("missing image URL")
	}
	outBytes, mime, err := parseDataURI(imgRef.ImageURL.URL)
	if err != nil {
		// Could be an HTTP URL instead of data URI — fetch it
		outBytes, mime, err = fetchImage(ctx, p.cfg.Client, imgRef.ImageURL.URL)
		if err != nil {
			return providers.Result{}, fmt.Errorf("decode output image: %w", err)
		}
	}
	return providers.Result{
		OutputImageBytes:  outBytes,
		OutputContentType: mime,
		ProviderRequestID: out.ID,
		EstimatedCostUSD:  estimatedCostUSD,
	}, nil
}

func parseDataURI(uri string) ([]byte, string, error) {
	if !strings.HasPrefix(uri, "data:") {
		return nil, "", errors.New("not a data URI")
	}
	parts := strings.SplitN(uri[5:], ",", 2)
	if len(parts) != 2 {
		return nil, "", errors.New("malformed data URI")
	}
	header := parts[0]
	data := parts[1]
	mime := header
	if semi := strings.Index(header, ";"); semi != -1 {
		mime = header[:semi]
	}
	if mime == "" {
		mime = "image/png"
	}
	b, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return nil, "", err
	}
	return b, mime, nil
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
