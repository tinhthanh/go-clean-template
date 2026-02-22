package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
)

type (
	// Config -.
	Config struct {
		App       App
		HTTP      HTTP
		Log       Log
		PG        PG
		GRPC      GRPC
		RMQ       RMQ
		NATS      NATS
		Redis     Redis
		Metrics   Metrics
		Swagger   Swagger
		CORS      CORS
		RateLimit RateLimit
		Tracer    Tracer
	}

	// App -.
	App struct {
		Name    string `env:"APP_NAME,required"`
		Version string `env:"APP_VERSION,required"`
		Env     string `env:"APP_ENV" envDefault:"development"`
	}

	// HTTP -.
	HTTP struct {
		Port           string `env:"HTTP_PORT,required"`
		UsePreforkMode bool   `env:"HTTP_USE_PREFORK_MODE" envDefault:"false"`
	}

	// Log -.
	Log struct {
		Level string `env:"LOG_LEVEL,required"`
	}

	// PG -.
	PG struct {
		PoolMax int    `env:"PG_POOL_MAX,required"`
		URL     string `env:"PG_URL,required"`
	}

	// GRPC -.
	GRPC struct {
		Enabled bool   `env:"GRPC_ENABLED" envDefault:"true"`
		Port    string `env:"GRPC_PORT" envDefault:"8081"`
	}

	// RMQ -.
	RMQ struct {
		Enabled        bool   `env:"RMQ_ENABLED" envDefault:"true"`
		ServerExchange string `env:"RMQ_RPC_SERVER" envDefault:"rpc_server"`
		ClientExchange string `env:"RMQ_RPC_CLIENT" envDefault:"rpc_client"`
		URL            string `env:"RMQ_URL" envDefault:"amqp://guest:guest@localhost:5672/"`
	}

	// NATS -.
	NATS struct {
		Enabled        bool   `env:"NATS_ENABLED" envDefault:"true"`
		ServerExchange string `env:"NATS_RPC_SERVER" envDefault:"rpc_server"`
		URL            string `env:"NATS_URL" envDefault:"nats://guest:guest@localhost:4222/"`
	}

	// Metrics -.
	Metrics struct {
		Enabled bool `env:"METRICS_ENABLED" envDefault:"true"`
	}

	// Swagger -.
	Swagger struct {
		Enabled bool `env:"SWAGGER_ENABLED" envDefault:"false"`
	}

	// CORS -.
	CORS struct {
		AllowOrigins string `env:"CORS_ALLOWED_ORIGINS" envDefault:"*"`
	}

	// RateLimit -.
	RateLimit struct {
		Max        int           `env:"RATE_LIMIT_MAX" envDefault:"100"`
		Expiration time.Duration `env:"RATE_LIMIT_EXPIRATION" envDefault:"1m"`
	}

	// Redis -.
	Redis struct {
		Enabled bool   `env:"REDIS_ENABLED" envDefault:"false"`
		URL     string `env:"REDIS_URL" envDefault:"redis://localhost:6379/0"`
	}

	// Tracer -.
	Tracer struct {
		Enabled     bool   `env:"TRACER_ENABLED" envDefault:"false"`
		URL         string `env:"TRACER_URL" envDefault:"http://localhost:4318/v1/traces"`
		ServiceName string `env:"TRACER_SERVICE_NAME" envDefault:"go-clean-template"`
	}
)

// NewConfig returns app config.
func NewConfig() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("config error: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("config validation error: %w", err)
	}

	return cfg, nil
}

// validate performs basic validation on the parsed config.
func (c *Config) validate() error {
	if c.PG.PoolMax <= 0 {
		return fmt.Errorf("PG_POOL_MAX must be positive, got %d", c.PG.PoolMax)
	}

	if c.HTTP.Port == "" {
		return fmt.Errorf("HTTP_PORT is required")
	}

	return nil
}

// IsProduction returns true if the app is running in production mode.
func (c *Config) IsProduction() bool {
	return c.App.Env == "production"
}
