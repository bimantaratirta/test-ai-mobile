package flux

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bimantara/ai-image/backend/internal/providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const onePixelPNG = "\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00\x1f\x15\xc4\x89\x00\x00\x00\rIDATx\x9cc\x00\x01\x00\x00\x05\x00\x01\r\n-\xb4\x00\x00\x00\x00IEND\xaeB`\x82"

func TestGenerate_FullFlow(t *testing.T) {
	imgServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte(onePixelPNG))
	}))
	defer imgServer.Close()

	var pollCount int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/predictions"):
			body, _ := io.ReadAll(r.Body)
			assert.Contains(t, string(body), "version")
			assert.Contains(t, string(body), "Industrial")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": "pred-123", "status": "starting",
			})
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/predictions/pred-123"):
			pollCount++
			status := "processing"
			var output any
			if pollCount >= 2 {
				status = "succeeded"
				output = []string{imgServer.URL + "/out.png"}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": "pred-123", "status": status, "output": output,
			})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	p := New(Config{Token: "test", BaseURL: srv.URL, ModelVersion: "v1", PollIntervalMs: 10})
	res, err := p.Generate(context.Background(), providers.Input{
		InputImageURL: "https://example.com/in.jpg",
		Prompt:        "Industrial coffee shop with exposed brick",
		StylePreset:   "industrial",
	})
	require.NoError(t, err)
	assert.Equal(t, "image/png", res.OutputContentType)
	assert.NotEmpty(t, res.OutputImageBytes)
	assert.Equal(t, "pred-123", res.ProviderRequestID)
	assert.InEpsilon(t, 0.04, res.EstimatedCostUSD, 0.0001)
}

func TestGenerate_Failed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "POST":
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "pred-x", "status": "starting"})
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": "pred-x", "status": "failed", "error": "model crashed",
			})
		}
	}))
	defer srv.Close()

	p := New(Config{Token: "test", BaseURL: srv.URL, ModelVersion: "v1", PollIntervalMs: 10})
	_, err := p.Generate(context.Background(), providers.Input{
		InputImageURL: "https://x", Prompt: "p",
	})
	require.Error(t, err)
}
