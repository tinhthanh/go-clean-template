package redis

import "time"

// Option is a functional option for Redis.
type Option func(*Redis)

// ConnAttempts sets the number of connection attempts.
func ConnAttempts(attempts int) Option {
	return func(r *Redis) {
		r.connAttempts = attempts
	}
}

// ConnTimeout sets the timeout between connection attempts.
func ConnTimeout(timeout time.Duration) Option {
	return func(r *Redis) {
		r.connTimeout = timeout
	}
}
