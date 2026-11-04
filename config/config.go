package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
)

type (
	// Config -.
	Config struct {
		App     app
		HTTP    http
		Log     log
		PG      pg
		GRPC    grpc
		RMQ     rmq
		NATS    nats
		JWT     jwt
		Metrics metrics
		Swagger swagger
		Tracing tracing
		CORS    cors
		Limit   limit
		API     api
	}

	// App -.
	app struct {
		Name    string `env:"APP_NAME,required"`
		Version string `env:"APP_VERSION,required"`
	}

	// HTTP -.
	http struct {
		Port           string `env:"HTTP_PORT,required"`
		UsePreforkMode bool   `env:"HTTP_USE_PREFORK_MODE" envDefault:"false"`
	}

	// Log -.
	log struct {
		Level string `env:"LOG_LEVEL,required"`
	}

	// PG -.
	pg struct {
		PoolMax int    `env:"PG_POOL_MAX,required"`
		URL     string `env:"PG_URL,required"`
	}

	// GRPC -.
	grpc struct {
		Enabled bool   `env:"GRPC_ENABLED" envDefault:"false"`
		Port    string `env:"GRPC_PORT,required"`
	}

	// RMQ -.
	rmq struct {
		Enabled        bool   `env:"RMQ_ENABLED" envDefault:"false"`
		ServerExchange string `env:"RMQ_RPC_SERVER,required"`
		ClientExchange string `env:"RMQ_RPC_CLIENT,required"`
		URL            string `env:"RMQ_URL,required"`
	}

	// NATS -.
	nats struct {
		Enabled        bool   `env:"NATS_ENABLED" envDefault:"false"`
		ServerExchange string `env:"NATS_RPC_SERVER,required"`
		URL            string `env:"NATS_URL,required"`
	}

	// JWT -.
	jwt struct {
		Secret      string        `env:"JWT_SECRET,required"`
		TokenExpiry time.Duration `env:"JWT_TOKEN_EXPIRY" envDefault:"24h"`
	}

	// Metrics -.
	metrics struct {
		Enabled bool `env:"METRICS_ENABLED" envDefault:"true"`
	}

	// Swagger -.
	swagger struct {
		Enabled bool `env:"SWAGGER_ENABLED" envDefault:"false"`
	}

	// Tracing -.
	tracing struct {
		Enabled      bool    `env:"TRACING_ENABLED" envDefault:"false"`
		OTLPEndpoint string  `env:"TRACING_OTLP_ENDPOINT" envDefault:"localhost:4317"`
		OTLPInsecure bool    `env:"TRACING_OTLP_INSECURE" envDefault:"true"`
		SampleRate   float64 `env:"TRACING_SAMPLE_RATE" envDefault:"0.1"`
	}

	// CORS -. The origins a browser may call the API from, comma-separated.
	cors struct {
		AllowOrigins []string `env:"CORS_ALLOWED_ORIGINS" envDefault:"*" envSeparator:","`
	}

	// API -. The client's contract (api/openapi.json) under Prefix, on the
	// session cookie: an access cookie of SessionLength and a refresh cookie
	// of RefreshLength. Secure is off only for plain HTTP in a test stack.
	api struct {
		Prefix        string        `env:"API_PREFIX"             envDefault:"/api"`
		CookieSecure  bool          `env:"SESSION_COOKIE_SECURE"  envDefault:"true"`
		SessionLength time.Duration `env:"SESSION_LENGTH"         envDefault:"8h"`
		RefreshLength time.Duration `env:"SESSION_REFRESH_LENGTH" envDefault:"168h"`
	}

	// Limit -. Requests per client in each window, kept in memory.
	limit struct {
		Max        int           `env:"RATE_LIMIT_MAX"        envDefault:"100"`
		Expiration time.Duration `env:"RATE_LIMIT_EXPIRATION" envDefault:"1m"`
	}
)

// NewConfig returns app config.
func NewConfig() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("config error: %w", err)
	}

	return cfg, nil
}
