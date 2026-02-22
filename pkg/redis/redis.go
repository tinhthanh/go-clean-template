// Package redis implements Redis connection with Options Pattern.
package redis

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	_defaultConnAttempts = 10
	_defaultConnTimeout  = time.Second
	_defaultMaxRetries   = 3
	_defaultDialTimeout  = 5 * time.Second
	_defaultReadTimeout  = 3 * time.Second
	_defaultWriteTimeout = 3 * time.Second
)

// Redis wraps a go-redis client with retry logic and options pattern.
type Redis struct {
	connAttempts int
	connTimeout  time.Duration

	Client *redis.Client
}

// New creates a new Redis connection with retry logic.
func New(url string, opts ...Option) (*Redis, error) {
	r := &Redis{
		connAttempts: _defaultConnAttempts,
		connTimeout:  _defaultConnTimeout,
	}

	for _, opt := range opts {
		opt(r)
	}

	redisOpts, err := redis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("redis - New - redis.ParseURL: %w", err)
	}

	redisOpts.MaxRetries = _defaultMaxRetries
	redisOpts.DialTimeout = _defaultDialTimeout
	redisOpts.ReadTimeout = _defaultReadTimeout
	redisOpts.WriteTimeout = _defaultWriteTimeout

	r.Client = redis.NewClient(redisOpts)

	// Retry connection.
	for r.connAttempts > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), r.connTimeout)

		err = r.Client.Ping(ctx).Err()

		cancel()

		if err == nil {
			break
		}

		log.Printf("redis is trying to connect, attempts left: %d", r.connAttempts)

		time.Sleep(r.connTimeout)

		r.connAttempts--
	}

	if err != nil {
		return nil, fmt.Errorf("redis - New - connAttempts == 0: %w", err)
	}

	return r, nil
}

// Close closes the Redis connection.
func (r *Redis) Close() error {
	if r.Client != nil {
		return r.Client.Close() //nolint:wrapcheck // passthrough
	}

	return nil
}

// Set stores a key-value pair with expiration.
func (r *Redis) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	return r.Client.Set(ctx, key, value, expiration).Err() //nolint:wrapcheck // passthrough
}

// Get retrieves a value by key. Returns redis.Nil if key does not exist.
func (r *Redis) Get(ctx context.Context, key string) (string, error) {
	return r.Client.Get(ctx, key).Result() //nolint:wrapcheck // passthrough
}

// Del removes one or more keys.
func (r *Redis) Del(ctx context.Context, keys ...string) error {
	return r.Client.Del(ctx, keys...).Err() //nolint:wrapcheck // passthrough
}

// Exists checks if a key exists. Returns the count of existing keys.
func (r *Redis) Exists(ctx context.Context, keys ...string) (int64, error) {
	return r.Client.Exists(ctx, keys...).Result() //nolint:wrapcheck // passthrough
}

// Ping checks the Redis connection.
func (r *Redis) Ping(ctx context.Context) error {
	return r.Client.Ping(ctx).Err() //nolint:wrapcheck // passthrough
}
