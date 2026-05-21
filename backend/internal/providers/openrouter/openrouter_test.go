package openrouter

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
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

// dualServer handles GET (image fetch) and POST (OpenRouter chat completions).
func dualServer(t *testing.T, postHandler http.HandlerFunc) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			w.Header().Set("Content-Type", "image/png")
			data, _ := base64.StdEncoding.DecodeString(tinyPNGB64)
			_, _ = w.Write(data)
			return
		}
		postHandler(w, r)
	}))
}

func TestGenerate_Success(t *testing.T) {
	srv := dualServer(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		assert.Contains(t, string(body), "google/gemini-2.5-flash-image")
		assert.Contains(t, string(body), "Scandinavian")
		_, _ = w.Write([]byte(fmt.Sprintf(`{
			"id": "gen-abc",
			"choices": [{
				"message": {
					"role": "assistant",
					"content": "",
					"images": [
						{"type": "image_url", "image_url": {"url": "data:image/png;base64,%s"}}
					]
				},
				"finish_reason": "stop"
			}]
		}`, tinyPNGB64)))
	})
	defer srv.Close()

	p := New(Config{
		APIKey:  "test",
		BaseURL: srv.URL,
		Model:   "google/gemini-2.5-flash-image",
	})
	res, err := p.Generate(context.Background(), providers.Input{
		InputImageURL: srv.URL + "/in.jpg",
		Prompt:        "Scandinavian coffee shop",
		StylePreset:   "scandinavian",
	})
	require.NoError(t, err)
	expected, _ := base64.StdEncoding.DecodeString(tinyPNGB64)
	assert.Equal(t, expected, res.OutputImageBytes)
	assert.Equal(t, "image/png", res.OutputContentType)
	assert.Equal(t, "gen-abc", res.ProviderRequestID)
	assert.Greater(t, res.EstimatedCostUSD, 0.0)
}

func TestGenerate_RateLimited(t *testing.T) {
	srv := dualServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"message":"rate limited","code":429}}`))
	})
	defer srv.Close()

	p := New(Config{APIKey: "test", BaseURL: srv.URL, Model: "google/gemini-2.5-flash-image"})
	_, err := p.Generate(context.Background(), providers.Input{
		InputImageURL: srv.URL + "/in.jpg",
		Prompt:        "five chars",
	})
	assert.ErrorIs(t, err, providers.ErrTransient)
}

func TestGenerate_ContentModerated(t *testing.T) {
	srv := dualServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{
				"message":       map[string]any{"role": "assistant", "content": ""},
				"finish_reason": "content_filter",
			}},
		})
	})
	defer srv.Close()

	p := New(Config{APIKey: "test", BaseURL: srv.URL, Model: "google/gemini-2.5-flash-image"})
	_, err := p.Generate(context.Background(), providers.Input{
		InputImageURL: srv.URL + "/in.jpg",
		Prompt:        "five chars",
	})
	assert.ErrorIs(t, err, providers.ErrContentModerated)
}

func TestGenerate_PromptBuiltCorrectly(t *testing.T) {
	var seenBody string
	srv := dualServer(t, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		seenBody = string(b)
		_, _ = w.Write([]byte(fmt.Sprintf(`{
			"id": "x",
			"choices": [{
				"message": {"role":"assistant","content":"","images":[{"type":"image_url","image_url":{"url":"data:image/png;base64,%s"}}]},
				"finish_reason": "stop"
			}]
		}`, tinyPNGB64)))
	})
	defer srv.Close()

	p := New(Config{APIKey: "test", BaseURL: srv.URL, Model: "google/gemini-2.5-flash-image"})
	_, err := p.Generate(context.Background(), providers.Input{
		InputImageURL: srv.URL + "/in.jpg",
		Prompt:        "Cozy with warm woods",
		StylePreset:   "japandi",
	})
	require.NoError(t, err)
	assert.True(t, strings.Contains(seenBody, "japandi"))
	assert.True(t, strings.Contains(seenBody, "Cozy with warm woods"))
	assert.True(t, strings.Contains(seenBody, "image_url"))
}
