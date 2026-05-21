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
	t.Setenv("R2_ACCOUNT_ID", "acc")
	t.Setenv("R2_ACCESS_KEY_ID", "akid")
	t.Setenv("R2_SECRET_ACCESS_KEY", "sk")
	t.Setenv("R2_BUCKET", "bkt")
	t.Setenv("R2_PUBLIC_BASE_URL", "https://cdn.test")
	t.Setenv("GEMINI_API_KEY", "gk")
	t.Setenv("REPLICATE_API_TOKEN", "rt")
	t.Setenv("REPLICATE_FLUX_VERSION", "v1")

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "9999", cfg.Port)
	assert.Equal(t, "test", cfg.Env)
	assert.Equal(t, "postgres://test", cfg.DatabaseURL)
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
