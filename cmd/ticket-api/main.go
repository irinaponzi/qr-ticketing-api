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
	"github.com/go-chi/chi/v5/middleware"
	"github.com/iponzi/entradasQR/internal/platform/config"
	"github.com/iponzi/entradasQR/internal/platform/database"
	"github.com/iponzi/entradasQR/internal/platform/metrics"
	appmiddleware "github.com/iponzi/entradasQR/internal/platform/middleware"
	"github.com/iponzi/entradasQR/internal/platform/rabbitmq"
	"github.com/iponzi/entradasQR/internal/ticket"
	ticketadapter "github.com/iponzi/entradasQR/internal/ticket/adapter"
	tickethandler "github.com/iponzi/entradasQR/internal/ticket/handler"
	ticketstorage "github.com/iponzi/entradasQR/internal/ticket/storage"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.LoadTicketAPIConfig()
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	db, err := database.NewMySQLConnection(cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName)
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

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

	// Repositories
	eventRepo := ticketstorage.NewMySQLEventRepository(db)
	ticketRepo := ticketstorage.NewMySQLTicketRepository(db)
	purchaseRepo := ticketstorage.NewMySQLPurchaseRepository(db)

	// Adapters
	publisher := ticketadapter.NewRabbitMQPublisher(rmqCh)

	// Service
	svc := ticket.NewTicketService(eventRepo, ticketRepo, purchaseRepo, publisher)

	// Consumer (ticket.used reconciliation from Validator)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	usedConsumer := ticketadapter.NewTicketUsedConsumer(rmqCh, svc, logger)
	if err := usedConsumer.StartConsuming(ctx); err != nil {
		logger.Error("failed to start ticket.used consumer", "error", err)
		os.Exit(1)
	}

	logger.Info("ticket.used consumer started")

	// Handler
	handler := tickethandler.NewTicketHandler(svc, eventRepo, logger)

	// Auth
	authValidator := appmiddleware.NewCognitoValidator(cfg.CognitoRegion, cfg.CognitoUserPoolID)

	// Router
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(metrics.HTTPMetricsMiddleware)
	r.Handle("/metrics", promhttp.Handler())
	r.Mount("/", handler.Routes(authValidator))

	addr := fmt.Sprintf(":%d", cfg.Port)
	logger.Info("ticket-api starting", "addr", addr)

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

	logger.Info("ticket-api stopped")
}
