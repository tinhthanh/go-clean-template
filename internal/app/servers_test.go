package app

import (
	"testing"
	"time"

	"github.com/evrone/go-clean-template/config"
	"github.com/evrone/go-clean-template/pkg/jwt"
	"github.com/evrone/go-clean-template/pkg/logger"
	"github.com/stretchr/testify/assert"
)

// A transport that is off is never built, so it needs no broker: the
// RabbitMQ and NATS addresses here answer nothing, and nothing dials them.
func TestInitServers_ATransportThatIsOffIsNeverStarted(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	cfg.HTTP.Port = "0"
	cfg.RMQ.URL = "amqp://nobody@127.0.0.1:1/"
	cfg.NATS.URL = "nats://127.0.0.1:1"

	s := initServers(cfg, &useCases{}, jwt.New("test-secret", time.Hour), logger.New("error"))

	assert.NotNil(t, s.http)
	assert.Nil(t, s.grpc)
	assert.Nil(t, s.rmq)
	assert.Nil(t, s.nats)
	assert.Nil(t, notify(s.grpc, s.grpc != nil))
}

func TestInitServers_GRPCIsBuiltWhenOn(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	cfg.HTTP.Port = "0"
	cfg.GRPC.Enabled = true
	cfg.GRPC.Port = "0"

	s := initServers(cfg, &useCases{}, jwt.New("test-secret", time.Hour), logger.New("error"))

	assert.NotNil(t, s.grpc)
	assert.NotNil(t, notify(s.grpc, true))
}
