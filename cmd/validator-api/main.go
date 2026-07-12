package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/iponzi/entradasQR/internal/platform/config"
	"github.com/iponzi/entradasQR/internal/platform/metrics"
	appmiddleware "github.com/iponzi/entradasQR/internal/platform/middleware"
	"github.com/iponzi/entradasQR/internal/platform/rabbitmq"
	ticketadapter "github.com/iponzi/entradasQR/internal/ticket/adapter"
	"github.com/iponzi/entradasQR/internal/validator"
	validatoradapter "github.com/iponzi/entradasQR/internal/validator/adapter"
	validatorhandler "github.com/iponzi/entradasQR/internal/validator/handler"
	validatorstorage "github.com/iponzi/entradasQR/internal/validator/storage"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.LoadValidatorAPIConfig()
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%d", cfg.RedisHost, cfg.RedisPort),
	})
	defer redisClient.Close()

	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		logger.Error("failed to connect to redis", "error", err)
		os.Exit(1)
	}

	rmqConn, err := rabbitmq.NewConnection(cfg.RabbitMQURL)
	if err != nil {
		logger.Error("failed to connect to rabbitmq", "error", err)
		os.Exit(1)
	}
	defer rmqConn.Close()

	rmqCh, err := rmqConn.Channel()
	if err != nil {
		logger.Error("failed to open rabbitmq channel", "error", err)
		os.Exit(1)
	}
	defer rmqCh.Close()

	if err := rabbitmq.SetupTopology(rmqCh); err != nil {
		logger.Error("failed to setup rabbitmq topology", "error", err)
		os.Exit(1)
	}

	// Repository (Redis-backed)
	validTicketRepo := validatorstorage.NewRedisValidTicketRepository(redisClient)

	// Adapters
	ticketClient := validatoradapter.NewTicketServiceHTTPClient(cfg.TicketServiceURL)
	eventPublisher := validatoradapter.NewRabbitMQPublisher(rmqCh)

	// Service
	svc := validator.NewValidatorService(validTicketRepo, ticketClient, eventPublisher, logger)

	// Consumer (event sync from RabbitMQ)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	consumer := validatoradapter.NewRabbitMQConsumer(rmqCh, svc, logger)
	if err := consumer.StartConsuming(ctx); err != nil {
		logger.Error("failed to start consuming", "error", err)
		os.Exit(1)
	}

	logger.Info("rabbitmq consumers started")

	// HTTP Handler
	tokenSigner := ticketadapter.NewHMACTokenSigner(cfg.HMACSecret)
	handler := validatorhandler.NewValidatorHandler(svc, tokenSigner, logger)

	// Auth
	authValidator := appmiddleware.NewCognitoValidator(cfg.CognitoRegion, cfg.CognitoUserPoolID)

	r := chi.NewRouter()
	rateLimiter := appmiddleware.NewIPRateLimiter(10, 20, logger)

	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.RequestID)
	r.Use(metrics.HTTPMetricsMiddleware)
	r.Use(rateLimiter.Middleware)
	r.Handle("/metrics", promhttp.Handler())
	r.Mount("/", handler.Routes(authValidator))

	addr := fmt.Sprintf(":%d", cfg.Port)
	logger.Info("validator-api starting", "addr", addr)

	srv := &http.Server{Addr: addr, Handler: r}

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		logger.Info("shutting down...")
		cancel()

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			logger.Error("server shutdown error", "error", err)
		}
	}()

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("server failed", "error", err)
		os.Exit(1)
	}

	logger.Info("validator-api stopped")
}
