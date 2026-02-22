// Package app configures and runs application.
package app

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/evrone/go-clean-template/config"
	amqprpc "github.com/evrone/go-clean-template/internal/controller/amqp_rpc"
	"github.com/evrone/go-clean-template/internal/controller/grpc"
	natsrpc "github.com/evrone/go-clean-template/internal/controller/nats_rpc"
	"github.com/evrone/go-clean-template/internal/controller/restapi"
	"github.com/evrone/go-clean-template/internal/repo/persistent"
	"github.com/evrone/go-clean-template/internal/repo/webapi"
	"github.com/evrone/go-clean-template/internal/usecase/translation"
	"github.com/evrone/go-clean-template/pkg/grpcserver"
	"github.com/evrone/go-clean-template/pkg/httpserver"
	"github.com/evrone/go-clean-template/pkg/logger"
	natsRPCServer "github.com/evrone/go-clean-template/pkg/nats/nats_rpc/server"
	"github.com/evrone/go-clean-template/pkg/postgres"
	rmqRPCServer "github.com/evrone/go-clean-template/pkg/rabbitmq/rmq_rpc/server"
	pkgredis "github.com/evrone/go-clean-template/pkg/redis"
	"github.com/evrone/go-clean-template/pkg/tracer"
)

const _shutdownTimeout = 5 * time.Second

// Run creates objects via constructors.
func Run(cfg *config.Config) { //nolint: gocyclo,cyclop,funlen,gocritic,nolintlint
	l := logger.New(cfg.Log.Level)

	l.Info("app - Run - starting %s v%s (env: %s)", cfg.App.Name, cfg.App.Version, cfg.App.Env)

	// OpenTelemetry Tracer (conditional)
	if cfg.Tracer.Enabled {
		tp, err := tracer.New(context.Background(), tracer.Config{
			ServiceName:    cfg.Tracer.ServiceName,
			ServiceVersion: cfg.App.Version,
			ExporterURL:    cfg.Tracer.URL,
			Environment:    cfg.App.Env,
		})
		if err != nil {
			l.Fatal(fmt.Errorf("app - Run - tracer.New: %w", err))
		}

		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), _shutdownTimeout)
			defer cancel()

			if err := tp.Shutdown(ctx); err != nil {
				l.Error(fmt.Errorf("app - Run - tracer.Shutdown: %w", err))
			}
		}()

		l.Info("app - Run - OpenTelemetry tracer initialized (exporter: %s)", cfg.Tracer.URL)
	} else {
		l.Info("app - Run - OpenTelemetry tracer disabled")
	}

	// Redis (conditional)
	if cfg.Redis.Enabled {
		rd, err := pkgredis.New(cfg.Redis.URL)
		if err != nil {
			l.Fatal(fmt.Errorf("app - Run - redis.New: %w", err))
		}

		defer func() {
			if err := rd.Close(); err != nil {
				l.Error(fmt.Errorf("app - Run - redis.Close: %w", err))
			}
		}()

		l.Info("app - Run - Redis connected")
	} else {
		l.Info("app - Run - Redis disabled")
	}

	// Repository
	pg, err := postgres.New(cfg.PG.URL, postgres.MaxPoolSize(cfg.PG.PoolMax))
	if err != nil {
		l.Fatal(fmt.Errorf("app - Run - postgres.New: %w", err))
	}
	defer pg.Close()

	// Use-Case
	translationUseCase := translation.New(
		persistent.New(pg),
		webapi.New(),
	)

	// RabbitMQ RPC Server (conditional)
	var rmqServer *rmqRPCServer.Server

	if cfg.RMQ.Enabled {
		rmqRouter := amqprpc.NewRouter(translationUseCase, l)

		rmqServer, err = rmqRPCServer.New(cfg.RMQ.URL, cfg.RMQ.ServerExchange, rmqRouter, l)
		if err != nil {
			l.Fatal(fmt.Errorf("app - Run - rmqServer - server.New: %w", err))
		}
	} else {
		l.Info("app - Run - RabbitMQ RPC server disabled")
	}

	// NATS RPC Server (conditional)
	var natsServer *natsRPCServer.Server

	if cfg.NATS.Enabled {
		natsRouter := natsrpc.NewRouter(translationUseCase, l)

		natsServer, err = natsRPCServer.New(cfg.NATS.URL, cfg.NATS.ServerExchange, natsRouter, l)
		if err != nil {
			l.Fatal(fmt.Errorf("app - Run - natsServer - server.New: %w", err))
		}
	} else {
		l.Info("app - Run - NATS RPC server disabled")
	}

	// gRPC Server (conditional)
	var grpcServer *grpcserver.Server

	if cfg.GRPC.Enabled {
		grpcServer = grpcserver.New(l, grpcserver.Port(cfg.GRPC.Port))
		grpc.NewRouter(grpcServer.App, translationUseCase, l)
	} else {
		l.Info("app - Run - gRPC server disabled")
	}

	// HTTP Server (always enabled)
	httpServer := httpserver.New(l, httpserver.Port(cfg.HTTP.Port), httpserver.Prefork(cfg.HTTP.UsePreforkMode))
	restapi.NewRouter(httpServer.App, cfg, translationUseCase, l)

	// Start servers
	if rmqServer != nil {
		rmqServer.Start()
	}

	if natsServer != nil {
		natsServer.Start()
	}

	if grpcServer != nil {
		grpcServer.Start()
	}

	httpServer.Start()

	// Waiting signal
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)

	select {
	case s := <-interrupt:
		l.Info("app - Run - signal: %s", s.String())
	case err = <-httpServer.Notify():
		l.Error(fmt.Errorf("app - Run - httpServer.Notify: %w", err))
	case err = <-notifyIfEnabled(grpcServer):
		l.Error(fmt.Errorf("app - Run - grpcServer.Notify: %w", err))
	case err = <-notifyIfEnabled(rmqServer):
		l.Error(fmt.Errorf("app - Run - rmqServer.Notify: %w", err))
	case err = <-notifyIfEnabled(natsServer):
		l.Error(fmt.Errorf("app - Run - natsServer.Notify: %w", err))
	}

	// Shutdown
	err = httpServer.Shutdown()
	if err != nil {
		l.Error(fmt.Errorf("app - Run - httpServer.Shutdown: %w", err))
	}

	if grpcServer != nil {
		err = grpcServer.Shutdown()
		if err != nil {
			l.Error(fmt.Errorf("app - Run - grpcServer.Shutdown: %w", err))
		}
	}

	if rmqServer != nil {
		err = rmqServer.Shutdown()
		if err != nil {
			l.Error(fmt.Errorf("app - Run - rmqServer.Shutdown: %w", err))
		}
	}

	if natsServer != nil {
		err = natsServer.Shutdown()
		if err != nil {
			l.Error(fmt.Errorf("app - Run - natsServer.Shutdown: %w", err))
		}
	}
}

// notifiable is an interface for servers that can notify errors.
type notifiable interface {
	Notify() <-chan error
}

// notifyIfEnabled returns the notify channel if the server is not nil,
// otherwise returns a channel that never receives.
func notifyIfEnabled[T notifiable](server T) <-chan error {
	var zero T
	if any(server) == any(zero) {
		return make(chan error) // never receives — blocks forever
	}

	return server.Notify()
}
