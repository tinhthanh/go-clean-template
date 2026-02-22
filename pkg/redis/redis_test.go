package redis_test

import (
	"testing"
	"time"

	pkgredis "github.com/evrone/go-clean-template/pkg/redis"
	"github.com/stretchr/testify/require"
)

func TestNew_InvalidURL(t *testing.T) {
	t.Parallel()

	_, err := pkgredis.New("invalid://url",
		pkgredis.ConnAttempts(1),
		pkgredis.ConnTimeout(100*time.Millisecond),
	)

	require.Error(t, err)
}

func TestNew_ConnectionFailure(t *testing.T) {
	t.Parallel()

	// Connect to a non-existent Redis server.
	_, err := pkgredis.New("redis://localhost:19999/0",
		pkgredis.ConnAttempts(1),
		pkgredis.ConnTimeout(100*time.Millisecond),
	)

	require.Error(t, err)
}

func TestOptions(t *testing.T) {
	t.Parallel()

	// Verify that options are applied without panic.
	// Cannot fully test without a running Redis instance,
	// but we validate that the constructor does not panic with valid options.
	_, err := pkgredis.New("redis://localhost:19999/0",
		pkgredis.ConnAttempts(1),
		pkgredis.ConnTimeout(50*time.Millisecond),
	)

	// Expected to fail because no Redis server is running on port 19999.
	require.Error(t, err)
}

func TestClose_NilClient(t *testing.T) {
	t.Parallel()

	r := &pkgredis.Redis{}
	err := r.Close()

	require.NoError(t, err)
}
