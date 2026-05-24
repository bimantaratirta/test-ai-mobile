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
	defaultBaseURL = "https://openrouter.ai/api/v1"
	// estimatedCostUSD approximates per-image cost. With Flex service tier
	// (the default below) Google charges ~50% of the standard rate, so the
	// effective cost is roughly half of the $0.039 standard rate.
	estimatedCostUSD = 0.020
	// Flex tier latency is variable (typically 20–90 s, sometimes longer
	// during peak). Give the HTTP client headroom over the worker timeout.
	requestTimeoutSec = 180
	// defaultServiceTier is OpenRouter's service tier hint. "flex" maps to
	// the provider's discounted-but-no-SLA tier (Google Vertex Flex).
	defaultServiceTier = "flex"
)

type Config struct {
	APIKey  string
	BaseURL string
	Model   string // e.g. "google/gemini-2.5-flash-image"
	Name    string // provider name reported via Name() — defaults to "openrouter"
	// ServiceTier overrides defaultServiceTier ("flex"). Set to "default"
	// or "priority" to opt into faster (and more expensive) processing.
	ServiceTier string
	// TwoStep, when true, runs a "clean room first" pass before the
	// renovation pass. Costs ~2× per generate but narrows each call's
	// task. Set false for single-pass behavior (cheaper, more drift).
	// Tests should set this explicitly; main wiring decides the default.
	TwoStep bool
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
	Model       string    `json:"model"`
	Messages    []message `json:"messages"`
	Modalities  []string  `json:"modalities,omitempty"`
	ServiceTier string    `json:"service_tier,omitempty"`
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

// modelOutput is what callModel returns: the generated image bytes, its
// mime type, and the request id for telemetry.
type modelOutput struct {
	bytes     []byte
	mime      string
	requestID string
}

// callModel sends a single (text + image) → image call to OpenRouter and
// returns the generated image. Shared by single-pass and two-step flows.
func (p *Provider) callModel(ctx context.Context, prompt string, imgBytes []byte, imgMime string) (modelOutput, error) {
	dataURI := fmt.Sprintf("data:%s;base64,%s", imgMime, base64.StdEncoding.EncodeToString(imgBytes))

	tier := p.cfg.ServiceTier
	if tier == "" {
		tier = defaultServiceTier
	}

	body := chatReq{
		Model: p.cfg.Model,
		Messages: []message{{
			Role: "user",
			Content: []messageContent{
				{Type: "text", Text: prompt},
				{Type: "image_url", ImageURL: &imageURL{URL: dataURI}},
			},
		}},
		Modalities:  []string{"image", "text"},
		ServiceTier: tier,
	}
	b, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, "POST", p.cfg.BaseURL+"/chat/completions", bytes.NewReader(b))
	if err != nil {
		return modelOutput{}, err
	}
	req.Header.Set("Authorization", "Bearer "+p.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("HTTP-Referer", "https://github.com/bimantara/ai-image")
	req.Header.Set("X-Title", "AI Image Coffee Shop Designer")

	resp, err := p.cfg.Client.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return modelOutput{}, providers.ErrTimeout
		}
		return modelOutput{}, fmt.Errorf("%w: %v", providers.ErrTransient, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return modelOutput{}, fmt.Errorf("%w: status %d", providers.ErrTransient, resp.StatusCode)
	}
	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(resp.Body)
		return modelOutput{}, fmt.Errorf("openrouter %d: %s", resp.StatusCode, string(raw))
	}

	var out chatResp
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return modelOutput{}, err
	}
	if out.Error != nil {
		if strings.Contains(strings.ToLower(out.Error.Message), "safety") ||
			strings.Contains(strings.ToLower(out.Error.Message), "filter") ||
			strings.Contains(strings.ToLower(out.Error.Message), "moderation") {
			return modelOutput{}, providers.ErrContentModerated
		}
		return modelOutput{}, fmt.Errorf("openrouter error: %s", out.Error.Message)
	}
	if len(out.Choices) == 0 {
		return modelOutput{}, errors.New("no choices returned")
	}
	ch := out.Choices[0]
	if ch.FinishReason == "content_filter" {
		return modelOutput{}, providers.ErrContentModerated
	}
	if len(ch.Message.Images) == 0 {
		return modelOutput{}, errors.New("no images in response")
	}
	imgRef := ch.Message.Images[0]
	if imgRef.ImageURL == nil || imgRef.ImageURL.URL == "" {
		return modelOutput{}, errors.New("missing image URL")
	}
	outBytes, mime, err := parseDataURI(imgRef.ImageURL.URL)
	if err != nil {
		// Could be an HTTP URL instead of data URI — fetch it
		outBytes, mime, err = fetchImage(ctx, p.cfg.Client, imgRef.ImageURL.URL)
		if err != nil {
			return modelOutput{}, fmt.Errorf("decode output image: %w", err)
		}
	}
	return modelOutput{bytes: outBytes, mime: mime, requestID: out.ID}, nil
}

func (p *Provider) Generate(ctx context.Context, in providers.Input) (providers.Result, error) {
	// Step 0: pull the input image once.
	imgBytes, imgMime, err := fetchImage(ctx, p.cfg.Client, in.InputImageURL)
	if err != nil {
		return providers.Result{}, fmt.Errorf("fetch input image: %w", err)
	}

	// RAW: prefix bypasses the entire two-step pipeline. It's an
	// exploration escape hatch — the user knows what they want, just
	// hand it to the model directly.
	if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(in.Prompt)), "RAW:") {
		out, err := p.callModel(ctx, strings.TrimSpace(strings.TrimSpace(in.Prompt)[4:]), imgBytes, imgMime)
		if err != nil {
			return providers.Result{}, err
		}
		return providers.Result{
			OutputImageBytes:  out.bytes,
			OutputContentType: out.mime,
			ProviderRequestID: out.requestID,
			EstimatedCostUSD:  estimatedCostUSD,
		}, nil
	}

	// Two-step pipeline:
	// 1. Clean the room (preserve walls/windows/doors/ceiling, drop clutter).
	// 2. Renovate the cleaned room.
	// Hypothesis: each call has a narrower task, so each is more faithful.
	// Trade-off: 2× cost and 2× latency.
	if p.cfg.TwoStep {
		// Step 1: clean.
		cleanOut, err := p.callModel(ctx, buildCleanPrompt(), imgBytes, imgMime)
		if err != nil {
			return providers.Result{}, fmt.Errorf("two-step: clean pass failed: %w", err)
		}
		// Step 2: renovate using the cleaned image as the new input.
		renoOut, err := p.callModel(ctx, buildPrompt(in.Prompt, in.StylePreset), cleanOut.bytes, cleanOut.mime)
		if err != nil {
			return providers.Result{}, fmt.Errorf("two-step: renovate pass failed: %w", err)
		}
		return providers.Result{
			OutputImageBytes:  renoOut.bytes,
			OutputContentType: renoOut.mime,
			ProviderRequestID: renoOut.requestID,
			EstimatedCostUSD:  estimatedCostUSD * 2, // two model calls
		}, nil
	}

	// Single-pass (legacy / tests).
	out, err := p.callModel(ctx, buildPrompt(in.Prompt, in.StylePreset), imgBytes, imgMime)
	if err != nil {
		return providers.Result{}, err
	}
	return providers.Result{
		OutputImageBytes:  out.bytes,
		OutputContentType: out.mime,
		ProviderRequestID: out.requestID,
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

// buildCleanPrompt is step 1 of the two-step pipeline. It instructs the
// model to remove clutter while preserving the room shell. The output is
// a "cleaned" version of the input that step 2 will then renovate.
func buildCleanPrompt() string {
	return "TASK: This photo shows a real room currently used for storage / " +
		"in mid-use, with miscellaneous items inside. Output ONE photograph " +
		"of the SAME ROOM, taken from the SAME camera position, with all " +
		"loose items, clutter, debris, boxes, bags, piles of clothing, " +
		"monitors, and random objects REMOVED. The room should appear empty " +
		"and freshly cleaned out, ready for renovation.\n\n" +
		"PRESERVE EXACTLY:\n" +
		"• Every wall — same count, same positions, same orientations, same texture and color.\n" +
		"• Room dimensions — width, depth, ceiling height unchanged.\n" +
		"• Windows — same count, size, shape, position. Do NOT add or remove.\n" +
		"• Doors — same count, size, position. Do NOT add or remove.\n" +
		"• Ceiling — same shape, same finish, same fixtures.\n" +
		"• Floor — same material and color (just bare and clean).\n" +
		"• Camera viewpoint — exact same vantage point, angle, and framing.\n" +
		"• Any architectural features (columns, beams, alcoves, wall " +
		"plates, light switches).\n\n" +
		"REMOVE: all loose contents — piles, boxes, plastic bags, debris, " +
		"random monitors, furniture that is clearly clutter, decorative " +
		"clutter, paper, fabric piles. Leave the room shell only.\n\n" +
		"OUTPUT: a single photorealistic photograph of this same empty " +
		"clean room from the same vantage point. No styling, no renovation " +
		"yet — just the bare clean shell."
}

func buildPrompt(userPrompt, stylePreset string) string {
	// Escape hatch: prompts prefixed with "RAW:" skip the coffee-shop
	// wrapper and pass straight to the model. Useful for exploring other
	// use cases (portrait edits, scene transforms, product viz, etc.).
	trimmed := strings.TrimSpace(userPrompt)
	if strings.HasPrefix(strings.ToUpper(trimmed), "RAW:") {
		// strip the RAW: prefix (case-insensitive) and surrounding whitespace
		return strings.TrimSpace(trimmed[4:])
	}

	style := strings.ToLower(stylePreset)
	if style == "" {
		style = "modern"
	}
	article := "a"
	switch style[0] {
	case 'a', 'e', 'i', 'o', 'u':
		article = "an"
	}
	return fmt.Sprintf(
		"TASK: Produce a photorealistic interior photograph of how THIS "+
			"EXACT room (already empty in the input) would look AFTER a %s "+
			"coffee shop renovation. The output and the input must look "+
			"like two photos taken at the same spot, with the same camera "+
			"and lens, of the same room — one bare, one renovated.\n\n"+
			"PRESERVE EXACTLY:\n"+
			"1. Walls — same count, positions, orientations, AND TEXTURE / "+
			"COLOR. Do not change wall finish unless the user explicitly "+
			"asks for new paint or paneling.\n"+
			"2. Room dimensions — width, depth, ceiling height must match. "+
			"Do NOT make the room larger or smaller.\n"+
			"3. Windows — same count, size, shape, wall, position. Do NOT "+
			"add or remove. If the input has no window, the output has no "+
			"window.\n"+
			"4. Doors — same count, position, type. Do NOT add or remove. "+
			"If the input has no door visible, the output has no door.\n"+
			"5. Camera viewpoint — same vantage, same lens, same framing. "+
			"Do NOT zoom in or out. Do NOT change the angle.\n"+
			"6. Architectural features — every column, beam, alcove, ledge, "+
			"wall outlet, light switch in the input stays in the output, "+
			"same place.\n\n"+
			"RENOVATE the room as %s %s coffee shop. Add: counter, seating, "+
			"tables, lighting fixtures (placed naturally), flooring finish "+
			"if relevant, decor, signage. Place these naturally INSIDE the "+
			"existing walls.\n\n"+
			"USER'S DESIGN VISION:\n%s\n\n"+
			"OUTPUT: a single photorealistic photograph indistinguishable "+
			"from a real photo of this exact room after the renovation.",
		style, article, style, userPrompt)
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
