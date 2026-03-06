package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/iponzi/entradasQR/internal/platform/config"
	"github.com/iponzi/entradasQR/internal/platform/rabbitmq"
	ticketadapter "github.com/iponzi/entradasQR/internal/ticket/adapter"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.LoadQRWorkerConfig()
	if err != nil {
		logger.Error("failed to load config", "error", err)
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

	qrGen := ticketadapter.NewQRCodeGenerator(cfg.QRSize)
	emailSender := ticketadapter.NewSMTPEmailSender(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPFrom, cfg.SMTPUser, cfg.SMTPPassword, logger)
	tokenSigner := ticketadapter.NewHMACTokenSigner(cfg.HMACSecret)
	consumer := ticketadapter.NewQRWorkerConsumer(rmqCh, qrGen, emailSender, tokenSigner, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		logger.Info("shutting down qr-worker...")
		cancel()
	}()

	logger.Info("qr-worker starting, consuming from " + rabbitmq.QueueQRWorker)

	if err := consumer.StartConsuming(ctx); err != nil {
		logger.Error("consumer failed", "error", err)
		os.Exit(1)
	}
}
