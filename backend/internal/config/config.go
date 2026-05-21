package config

import (
	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	Port string `envconfig:"PORT" default:"8080"`
	Env  string `envconfig:"ENV"  default:"development"`

	DatabaseURL            string `envconfig:"DATABASE_URL"               required:"true"`
	SupabaseURL            string `envconfig:"SUPABASE_URL"               required:"true"`
	SupabaseServiceRoleKey string `envconfig:"SUPABASE_SERVICE_ROLE_KEY"  required:"true"`
	SupabaseJWTSecret      string `envconfig:"SUPABASE_JWT_SECRET"        required:"true"`

	R2AccountID       string `envconfig:"R2_ACCOUNT_ID"        required:"true"`
	R2AccessKeyID     string `envconfig:"R2_ACCESS_KEY_ID"     required:"true"`
	R2SecretAccessKey string `envconfig:"R2_SECRET_ACCESS_KEY" required:"true"`
	R2Bucket          string `envconfig:"R2_BUCKET"            required:"true"`
	R2PublicBaseURL   string `envconfig:"R2_PUBLIC_BASE_URL"   required:"true"`

	GeminiAPIKey         string `envconfig:"GEMINI_API_KEY"          required:"true"`
	ReplicateAPIToken    string `envconfig:"REPLICATE_API_TOKEN"     required:"true"`
	ReplicateFluxVersion string `envconfig:"REPLICATE_FLUX_VERSION"  required:"true"`

	SentryDSN string `envconfig:"SENTRY_DSN" default:""`

	RateLimitPerDay          int     `envconfig:"RATE_LIMIT_PER_DAY"           default:"10"`
	RateLimitPerWeek         int     `envconfig:"RATE_LIMIT_PER_WEEK"          default:"30"`
	GlobalCostCapUSD         float64 `envconfig:"GLOBAL_COST_CAP_USD"          default:"25.00"`
	MaxConcurrentJobsPerUser int     `envconfig:"MAX_CONCURRENT_JOBS_PER_USER" default:"3"`
}

func Load() (*Config, error) {
	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
