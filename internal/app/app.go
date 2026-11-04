// Package app configures and runs application.
package app

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/evrone/go-clean-template/api"
	"github.com/evrone/go-clean-template/config"
	amqprpc "github.com/evrone/go-clean-template/internal/controller/amqp_rpc"
	"github.com/evrone/go-clean-template/internal/controller/grpc"
	grpcmw "github.com/evrone/go-clean-template/internal/controller/grpc/middleware"
	natsrpc "github.com/evrone/go-clean-template/internal/controller/nats_rpc"
	"github.com/evrone/go-clean-template/internal/controller/restapi"
	"github.com/evrone/go-clean-template/internal/controller/restapi/clientapi"
	persistAuditRepo "github.com/evrone/go-clean-template/internal/repo/persistent/audit"
	persistPatientRepo "github.com/evrone/go-clean-template/internal/repo/persistent/patient"
	persistPrincipalRepo "github.com/evrone/go-clean-template/internal/repo/persistent/principal"
	persistTaskRepo "github.com/evrone/go-clean-template/internal/repo/persistent/task"
	persistTranslationRepo "github.com/evrone/go-clean-template/internal/repo/persistent/translation"
	persistUserRepo "github.com/evrone/go-clean-template/internal/repo/persistent/user"
	"github.com/evrone/go-clean-template/internal/repo/webapi"
	"github.com/evrone/go-clean-template/internal/usecase"
	"github.com/evrone/go-clean-template/internal/usecase/audit"
	"github.com/evrone/go-clean-template/internal/usecase/patient"
	"github.com/evrone/go-clean-template/internal/usecase/session"
	"github.com/evrone/go-clean-template/internal/usecase/task"
	"github.com/evrone/go-clean-template/internal/usecase/translation"
	"github.com/evrone/go-clean-template/internal/usecase/user"
	"github.com/evrone/go-clean-template/pkg/contract"
	"github.com/evrone/go-clean-template/pkg/grpcserver"
	"github.com/evrone/go-clean-template/pkg/httpserver"
	"github.com/evrone/go-clean-template/pkg/jwt"
	"github.com/evrone/go-clean-template/pkg/logger"
	natsRPCServer "github.com/evrone/go-clean-template/pkg/nats/nats_rpc/server"
	"github.com/evrone/go-clean-template/pkg/postgres"
	rmqRPCServer "github.com/evrone/go-clean-template/pkg/rabbitmq/rmq_rpc/server"
	"github.com/evrone/go-clean-template/pkg/tracing"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	pbgrpc "google.golang.org/grpc"
)

type useCases struct {
	translation usecase.Translation
	user        usecase.User
	task        usecase.Task
	session     usecase.Session
	audit       usecase.Audit
	patient     usecase.Patient
}

type servers struct {
	rmq  *rmqRPCServer.Server
	nats *natsRPCServer.Server
	grpc *grpcserver.Server
	http *httpserver.Server
}

func initUseCases(pg *postgres.Postgres, jwtManager *jwt.Manager) useCases {
	translationRepo := persistTranslationRepo.New(pg)
	taskRepo := persistTaskRepo.New(pg)
	userRepo := persistUserRepo.New(pg)

	return useCases{
		user:        user.New(userRepo, jwtManager),
		task:        task.New(taskRepo),
		translation: translation.New(translationRepo, webapi.New()),
		session:     session.New(persistPrincipalRepo.New(pg)),
		audit:       audit.New(persistAuditRepo.New(pg)),
		patient:     patient.New(persistPatientRepo.New(pg)),
	}
}

// initServers starts the HTTP server always, and gRPC, RabbitMQ RPC and NATS
// RPC only when their *_ENABLED says so: a disabled one stays nil, and needs
// no broker running.
func initServers(cfg *config.Config, uc *useCases, jwtManager *jwt.Manager, l logger.Interface) servers {
	var s servers

	if cfg.RMQ.Enabled {
		rmqRouter := amqprpc.NewRouter(uc.translation, uc.user, uc.task, jwtManager, l)

		rmqServer, err := rmqRPCServer.New(cfg.RMQ.URL, cfg.RMQ.ServerExchange, rmqRouter, l)
		if err != nil {
			l.Fatal(fmt.Errorf("app - Run - rmqServer - server.New: %w", err))
		}

		s.rmq = rmqServer
	}

	if cfg.NATS.Enabled {
		natsRouter := natsrpc.NewRouter(uc.translation, uc.user, uc.task, jwtManager, l)

		natsServer, err := natsRPCServer.New(cfg.NATS.URL, cfg.NATS.ServerExchange, natsRouter, l)
		if err != nil {
			l.Fatal(fmt.Errorf("app - Run - natsServer - server.New: %w", err))
		}

		s.nats = natsServer
	}

	if cfg.GRPC.Enabled {
		s.grpc = grpcserver.New(
			l,
			grpcserver.Port(cfg.GRPC.Port),
			grpcserver.ServerOptions(
				pbgrpc.UnaryInterceptor(grpcmw.AuthInterceptor(jwtManager)),
				pbgrpc.StatsHandler(otelgrpc.NewServerHandler()),
			),
		)
		grpc.NewRouter(s.grpc.App, uc.translation, uc.user, uc.task, l)
	}

	s.http = httpserver.New(l, httpserver.Port(cfg.HTTP.Port), httpserver.Prefork(cfg.HTTP.UsePreforkMode))
	restapi.NewRouter(s.http.App, cfg, uc.translation, uc.user, uc.task, jwtManager, l)

	if _, err := clientapi.NewRoutes(s.http.App.Group(cfg.API.Prefix), clientDeps(cfg, uc, l)); err != nil {
		l.Fatal(fmt.Errorf("app - Run - clientapi.NewRoutes: %w", err))
	}

	return s
}

// clientDeps is the client's contract (api/openapi.json) and what serves
// it. The session and refresh cookies are signed with secrets of their own,
// derived from JWT_SECRET, so neither passes for the other or for /v1's token.
func clientDeps(cfg *config.Config, uc *useCases, l logger.Interface) *clientapi.Deps {
	spec, err := contract.Load(api.Spec())
	if err != nil {
		l.Fatal(fmt.Errorf("app - Run - contract.Load: %w", err))
	}

	return &clientapi.Deps{
		Contract: spec,
		Session:  uc.session,
		Audit:    uc.audit,
		Patient:  uc.patient,
		Access:   jwt.New(cfg.JWT.Secret+":session", cfg.API.SessionLength),
		Refresh:  jwt.New(cfg.JWT.Secret+":refresh", cfg.API.RefreshLength),
		Cookies: clientapi.Cookies{
			Prefix: cfg.API.Prefix, Secure: cfg.API.CookieSecure,
			SessionLength: cfg.API.SessionLength, RefreshLength: cfg.API.RefreshLength,
		},
		Logger: l,
	}
}

func (s *servers) startServers() {
	if s.rmq != nil {
		s.rmq.Start()
	}

	if s.nats != nil {
		s.nats.Start()
	}

	if s.grpc != nil {
		s.grpc.Start()
	}

	s.http.Start()
}

// notify is a server's error channel, or nil (which never fires) for one that is off.
func notify[T interface{ Notify() <-chan error }](srv T, on bool) <-chan error {
	if !on {
		return nil
	}

	return srv.Notify()
}

func (s *servers) waitForShutdown(l logger.Interface) {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)

	var err error

	select {
	case sig := <-interrupt:
		l.Info("app - Run - signal: %s", sig.String())
	case err = <-s.http.Notify():
		l.Error(fmt.Errorf("app - Run - httpServer.Notify: %w", err))
	case err = <-notify(s.grpc, s.grpc != nil):
		l.Error(fmt.Errorf("app - Run - grpcServer.Notify: %w", err))
	case err = <-notify(s.rmq, s.rmq != nil):
		l.Error(fmt.Errorf("app - Run - rmqServer.Notify: %w", err))
	case err = <-notify(s.nats, s.nats != nil):
		l.Error(fmt.Errorf("app - Run - natsServer.Notify: %w", err))
	}

	s.shutdownServers(l)
}

func (s *servers) shutdownServers(l logger.Interface) {
	if err := s.http.Shutdown(); err != nil {
		l.Error(fmt.Errorf("app - Run - httpServer.Shutdown: %w", err))
	}

	if s.grpc != nil {
		if err := s.grpc.Shutdown(); err != nil {
			l.Error(fmt.Errorf("app - Run - grpcServer.Shutdown: %w", err))
		}
	}

	if s.rmq != nil {
		if err := s.rmq.Shutdown(); err != nil {
			l.Error(fmt.Errorf("app - Run - rmqServer.Shutdown: %w", err))
		}
	}

	if s.nats != nil {
		if err := s.nats.Shutdown(); err != nil {
			l.Error(fmt.Errorf("app - Run - natsServer.Shutdown: %w", err))
		}
	}
}

// Run creates objects via constructors.
func Run(cfg *config.Config) {
	l := logger.New(cfg.Log.Level)

	ctx := context.Background()

	// Tracing
	shutdownTracing, err := tracing.New(ctx, tracing.Config{
		Enabled:     cfg.Tracing.Enabled,
		ServiceName: cfg.App.Name,
		Version:     cfg.App.Version,
		Endpoint:    cfg.Tracing.OTLPEndpoint,
		Insecure:    cfg.Tracing.OTLPInsecure,
		SampleRate:  cfg.Tracing.SampleRate,
	})
	if err != nil {
		l.Fatal(fmt.Errorf("app - Run - tracing.New: %w", err))
	}
	defer func() {
		if err := shutdownTracing(ctx); err != nil {
			l.Error(fmt.Errorf("app - Run - shutdownTracing: %w", err))
		}
	}()

	// Repository
	pg, err := postgres.New(cfg.PG.URL, postgres.MaxPoolSize(cfg.PG.PoolMax))
	if err != nil {
		l.Fatal(fmt.Errorf("app - Run - postgres.New: %w", err))
	}
	defer pg.Close()

	// JWT
	jwtManager := jwt.New(cfg.JWT.Secret, cfg.JWT.TokenExpiry)

	uc := initUseCases(pg, jwtManager)
	s := initServers(cfg, &uc, jwtManager, l)
	s.startServers()
	s.waitForShutdown(l)
}
