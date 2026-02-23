//go:build integration

package redis_test

import (
	"context"
	"os"
	"testing"
	"time"

	pkgredis "github.com/evrone/go-clean-template/pkg/redis"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
)

const redisImage = "redis:7.4-alpine"

type RedisSuite struct {
	suite.Suite
	container *tcredis.RedisContainer // nil when using external Redis
	client    *pkgredis.Redis
	ctx       context.Context
}

func (s *RedisSuite) SetupSuite() {
	s.ctx = context.Background()

	var connStr string

	// If TEST_REDIS_URL is set (Docker compose), use it directly.
	// Otherwise, spin up a testcontainer for local development.
	if redisURL := os.Getenv("TEST_REDIS_URL"); redisURL != "" {
		connStr = redisURL
	} else {
		container, err := tcredis.Run(s.ctx, redisImage)
		require.NoError(s.T(), err)

		s.container = container

		cs, err := container.ConnectionString(s.ctx)
		require.NoError(s.T(), err)

		connStr = cs
	}

	// Connect to Redis.
	client, err := pkgredis.New(connStr,
		pkgredis.ConnAttempts(10),
		pkgredis.ConnTimeout(time.Second),
	)
	require.NoError(s.T(), err)

	s.client = client
}

func (s *RedisSuite) TearDownSuite() {
	if s.client != nil {
		err := s.client.Close()
		require.NoError(s.T(), err)
	}

	if s.container != nil {
		err := s.container.Terminate(s.ctx)
		require.NoError(s.T(), err)
	}
}

func (s *RedisSuite) SetupTest() {
	// Flush all keys before each test.
	err := s.client.Client.FlushAll(s.ctx).Err()
	require.NoError(s.T(), err)
}

func (s *RedisSuite) TestPing() {
	err := s.client.Ping(s.ctx)
	require.NoError(s.T(), err)
}

func (s *RedisSuite) TestSetAndGet() {
	err := s.client.Set(s.ctx, "greeting", "xin chào", 0)
	require.NoError(s.T(), err)

	val, err := s.client.Get(s.ctx, "greeting")
	require.NoError(s.T(), err)
	require.Equal(s.T(), "xin chào", val)
}

func (s *RedisSuite) TestGetNonExistent() {
	_, err := s.client.Get(s.ctx, "non_existent_key")
	require.ErrorIs(s.T(), err, redis.Nil)
}

func (s *RedisSuite) TestDel() {
	err := s.client.Set(s.ctx, "to_delete", "value", 0)
	require.NoError(s.T(), err)

	err = s.client.Del(s.ctx, "to_delete")
	require.NoError(s.T(), err)

	_, err = s.client.Get(s.ctx, "to_delete")
	require.ErrorIs(s.T(), err, redis.Nil)
}

func (s *RedisSuite) TestExists() {
	err := s.client.Set(s.ctx, "exists_key", "value", 0)
	require.NoError(s.T(), err)

	count, err := s.client.Exists(s.ctx, "exists_key")
	require.NoError(s.T(), err)
	require.Equal(s.T(), int64(1), count)

	count, err = s.client.Exists(s.ctx, "missing_key")
	require.NoError(s.T(), err)
	require.Equal(s.T(), int64(0), count)
}

func (s *RedisSuite) TestTTLExpiration() {
	err := s.client.Set(s.ctx, "expiring_key", "temp", 200*time.Millisecond)
	require.NoError(s.T(), err)

	// Key should exist immediately.
	val, err := s.client.Get(s.ctx, "expiring_key")
	require.NoError(s.T(), err)
	require.Equal(s.T(), "temp", val)

	// Wait for expiration.
	time.Sleep(300 * time.Millisecond)

	_, err = s.client.Get(s.ctx, "expiring_key")
	require.ErrorIs(s.T(), err, redis.Nil)
}

func (s *RedisSuite) TestSetOverwritesExistingKey() {
	err := s.client.Set(s.ctx, "overwrite_key", "original", 0)
	require.NoError(s.T(), err)

	err = s.client.Set(s.ctx, "overwrite_key", "updated", 0)
	require.NoError(s.T(), err)

	val, err := s.client.Get(s.ctx, "overwrite_key")
	require.NoError(s.T(), err)
	require.Equal(s.T(), "updated", val)
}

func TestRedisSuite(t *testing.T) {
	suite.Run(t, new(RedisSuite))
}
