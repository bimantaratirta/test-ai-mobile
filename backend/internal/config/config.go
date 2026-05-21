package config

import (
	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	Port string `envconfig:"PORT" default:"8080"`
	Env  string `envconfig:"ENV"  default:"development"`

	DatabaseURL              string `envconfig:"DATABASE_URL"               required:"true"`
	SupabaseURL              string `envconfig:"SUPABASE_URL"               required:"true"`
	SupabaseServiceRoleKey   string `envconfig:"SUPABASE_SERVICE_ROLE_KEY"  required:"true"`
	SupabaseJWTSecret        string `envconfig:"SUPABASE_JWT_SECRET"        required:"true"`

	S3Endpoint        string `envconfig:"S3_ENDPOINT"          required:"true"`
	S3Region          string `envconfig:"S3_REGION"            default:"auto"`
	S3AccessKeyID     string `envconfig:"S3_ACCESS_KEY_ID"     required:"true"`
	S3SecretAccessKey string `envconfig:"S3_SECRET_ACCESS_KEY" required:"true"`
	S3Bucket          string `envconfig:"S3_BUCKET"            required:"true"`
	S3PublicBaseURL   string `envconfig:"S3_PUBLIC_BASE_URL"   required:"true"`

	OpenRouterAPIKey             string `envconfig:"OPENROUTER_API_KEY"              required:"true"`
	OpenRouterRealisticModel     string `envconfig:"OPENROUTER_REALISTIC_MODEL"      default:"google/gemini-2.5-flash-image-preview"`
	OpenRouterInspirationalModel string `envconfig:"OPENROUTER_INSPIRATIONAL_MODEL"  default:""`

	ReplicateAPIToken    string `envconfig:"REPLICATE_API_TOKEN"     default:""`
	ReplicateFluxVersion string `envconfig:"REPLICATE_FLUX_VERSION"  default:""`

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
