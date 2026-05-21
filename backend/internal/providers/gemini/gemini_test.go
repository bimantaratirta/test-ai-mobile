package gemini

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bimantara/ai-image/backend/internal/providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const tinyPNGB64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkYAAAAAYAAjCB0C8AAAAASUVORK5CYII="

// tinyPNGBytes is the decoded form of the tiny 1x1 PNG.
var tinyPNGBytes, _ = base64.StdEncoding.DecodeString(tinyPNGB64)

// newDualServer creates a single httptest.Server that:
//   - On GET  → serves tinyPNGBytes as image/png (simulates the input image URL)
//   - On POST → calls apiHandler (simulates the Gemini API endpoint)
func newDualServer(t *testing.T, apiHandler http.HandlerFunc) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(tinyPNGBytes)
		case http.MethodPost:
			apiHandler(w, r)
		default:
			http.Error(w, "unexpected method", http.StatusMethodNotAllowed)
		}
	}))
}

func TestGenerate_Success(t *testing.T) {
	srv := newDualServer(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		assert.Contains(t, string(body), "Scandinavian")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"candidates": [{
				"content": {
					"parts": [{
						"inlineData": {
							"mimeType": "image/png",
							"data": "` + tinyPNGB64 + `"
						}
					}]
				}
			}]
		}`))
	})
	defer srv.Close()

	p := New(Config{APIKey: "test", BaseURL: srv.URL})
	res, err := p.Generate(context.Background(), providers.Input{
		InputImageURL: srv.URL + "/in.jpg",
		Prompt:        "Scandinavian coffee shop",
		StylePreset:   "scandinavian",
	})
	require.NoError(t, err)
	expected, _ := base64.StdEncoding.DecodeString(tinyPNGB64)
	assert.Equal(t, expected, res.OutputImageBytes)
	assert.Equal(t, "image/png", res.OutputContentType)
	assert.Greater(t, res.EstimatedCostUSD, 0.0)
}

func TestGenerate_RateLimited(t *testing.T) {
	srv := newDualServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":"rate limited"}`))
	})
	defer srv.Close()

	p := New(Config{APIKey: "test", BaseURL: srv.URL})
	_, err := p.Generate(context.Background(), providers.Input{
		InputImageURL: srv.URL + "/in.jpg",
		Prompt:        "five chars",
	})
	assert.ErrorIs(t, err, providers.ErrTransient)
}

func TestGenerate_ContentModerated(t *testing.T) {
	srv := newDualServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"candidates": [{
				"finishReason": "SAFETY",
				"content": {"parts": []}
			}]
		}`))
	})
	defer srv.Close()

	p := New(Config{APIKey: "test", BaseURL: srv.URL})
	_, err := p.Generate(context.Background(), providers.Input{
		InputImageURL: srv.URL + "/in.jpg",
		Prompt:        "five chars",
	})
	assert.ErrorIs(t, err, providers.ErrContentModerated)
}

func TestGenerate_PromptIncludesStyleAndImage(t *testing.T) {
	var seenBody string
	srv := newDualServer(t, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		seenBody = string(b)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"inlineData":{"mimeType":"image/png","data":"` + tinyPNGB64 + `"}}]}}]}`))
	})
	defer srv.Close()

	p := New(Config{APIKey: "test", BaseURL: srv.URL})
	_, err := p.Generate(context.Background(), providers.Input{
		InputImageURL: srv.URL + "/in.jpg",
		Prompt:        "Cozy with warm woods",
		StylePreset:   "japandi",
	})
	require.NoError(t, err)
	assert.True(t, strings.Contains(seenBody, "japandi"))
	assert.True(t, strings.Contains(seenBody, "Cozy with warm woods"))
	// Image should be referenced (either as URL or fetched and base64'd)
	assert.True(t,
		strings.Contains(seenBody, "in.jpg") || strings.Contains(seenBody, "inlineData"),
		"request should reference the input image")
}
