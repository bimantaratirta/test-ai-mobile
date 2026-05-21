package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_FromEnv(t *testing.T) {
	t.Setenv("PORT", "9999")
	t.Setenv("ENV", "test")
	t.Setenv("DATABASE_URL", "postgres://test")
	t.Setenv("SUPABASE_URL", "https://x.supabase.co")
	t.Setenv("SUPABASE_SERVICE_ROLE_KEY", "srk")
	t.Setenv("SUPABASE_JWT_SECRET", "jwts")
	t.Setenv("S3_ENDPOINT", "https://is3.cloudhost.id")
	t.Setenv("S3_ACCESS_KEY_ID", "akid")
	t.Setenv("S3_SECRET_ACCESS_KEY", "sk")
	t.Setenv("S3_BUCKET", "bkt")
	t.Setenv("S3_PUBLIC_BASE_URL", "https://cdn.test")
	t.Setenv("OPENROUTER_API_KEY", "or-key")

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "9999", cfg.Port)
	assert.Equal(t, "test", cfg.Env)
	assert.Equal(t, "https://is3.cloudhost.id", cfg.S3Endpoint)
	assert.Equal(t, "auto", cfg.S3Region) // default
	assert.Equal(t, "or-key", cfg.OpenRouterAPIKey)
	assert.Equal(t, "google/gemini-2.5-flash-image-preview", cfg.OpenRouterRealisticModel) // default
	assert.Equal(t, "", cfg.OpenRouterInspirationalModel)                                  // default
	assert.Equal(t, "", cfg.ReplicateAPIToken)                                             // optional
	assert.Equal(t, 10, cfg.RateLimitPerDay)
	assert.Equal(t, 30, cfg.RateLimitPerWeek)
	assert.InEpsilon(t, 25.0, cfg.GlobalCostCapUSD, 0.0001)
	assert.Equal(t, 3, cfg.MaxConcurrentJobsPerUser)
}

func TestLoad_MissingRequired(t *testing.T) {
	os.Clearenv()
	_, err := Load()
	require.Error(t, err)
}
