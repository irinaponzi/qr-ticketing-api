package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/iponzi/entradasQR/internal/platform/metrics"
	"github.com/iponzi/entradasQR/internal/platform/rabbitmq"
	"github.com/iponzi/entradasQR/internal/ticket"
	amqp "github.com/rabbitmq/amqp091-go"
)

// QRWorkerConsumer consumes purchase.completed events from RabbitMQ,
// generates QR code images for each ticket, and sends a confirmation
// email to the buyer. It acts as an adapter bridging the message broker
// with the QR generation and email delivery ports.
type QRWorkerConsumer struct {
	channel     *amqp.Channel
	qrGenerator ticket.QRGenerator
	emailSender ticket.EmailSender
	tokenSigner ticket.TokenSigner
	logger      *slog.Logger
}

// NewQRWorkerConsumer creates a new QRWorkerConsumer with the given dependencies.
//
// Parameters:
//   - ch: An open AMQP channel for consuming messages.
//   - qrGenerator: Generator for QR code images from ticket codes.
//   - emailSender: Sender for ticket confirmation emails with QR attachments.
//   - tokenSigner: Signer for HMAC-signing ticket codes before QR encoding.
//   - logger: Structured logger for observability.
//
// Returns:
//   - *QRWorkerConsumer: A pointer to the newly created consumer.
func NewQRWorkerConsumer(
	ch *amqp.Channel,
	qrGenerator ticket.QRGenerator,
	emailSender ticket.EmailSender,
	tokenSigner ticket.TokenSigner,
	logger *slog.Logger,
) *QRWorkerConsumer {
	return &QRWorkerConsumer{
		channel:     ch,
		qrGenerator: qrGenerator,
		emailSender: emailSender,
		tokenSigner: tokenSigner,
		logger:      logger,
	}
}

// StartConsuming begins listening on the purchase.completed queue and processes
// each message by generating QR codes and sending the confirmation email.
// It blocks until the context is cancelled or the channel is closed.
//
// Parameters:
//   - ctx: The context for cancellation; when cancelled, the consumer stops.
//
// Returns:
//   - error: A wrapped error if registering the consumer fails or the channel
//     closes unexpectedly; nil if the context is cancelled gracefully.
func (c *QRWorkerConsumer) StartConsuming(ctx context.Context) error {
	msgs, err := c.channel.Consume(
		rabbitmq.QueueQRWorker,
		"qr-worker",
		false, // manual ack
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("registering consumer: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			c.logger.Info("context cancelled, stopping qr-worker consumer")
			return nil
		case msg, ok := <-msgs:
			if !ok {
				return fmt.Errorf("rabbitmq channel closed")
			}

			c.HandleMessage(ctx, msg)
		}
	}
}

// HandleMessage processes a single RabbitMQ delivery containing a
// PurchaseCompletedEvent. It generates QR codes for each ticket and sends
// the confirmation email. The message is acknowledged on success, rejected
// without requeue on unmarshal errors, and rejected with requeue on
// transient failures (e.g., SMTP errors).
//
// Parameters:
//   - ctx: The request context for cancellation and tracing.
//   - msg: The RabbitMQ delivery to process.
func (c *QRWorkerConsumer) HandleMessage(ctx context.Context, msg amqp.Delivery) {
	var event ticket.PurchaseCompletedEvent
	if err := json.Unmarshal(msg.Body, &event); err != nil {
		c.logger.Error("failed to unmarshal message", "error", err)
		msg.Nack(false, false)

		return
	}

	c.logger.Info("processing purchase",
		"purchase_id", event.PurchaseID,
		"buyer_email", event.BuyerEmail,
		"tickets", len(event.TicketCodes),
	)

	qrImages := make([][]byte, 0, len(event.TicketCodes))

	for _, code := range event.TicketCodes {
		token := c.tokenSigner.Sign(code)

		img, err := c.qrGenerator.Generate(token)
		if err != nil {
			c.logger.Error("failed to generate QR", "code", code, "error", err)

			continue
		}

		qrImages = append(qrImages, img)
	}

	if err := c.emailSender.SendTicketEmail(ctx, event.BuyerEmail, event.EventName, qrImages); err != nil {
		c.logger.Error("failed to send email, will retry", "email", event.BuyerEmail, "error", err)
		msg.Nack(false, true)

		return
	}

	msg.Ack(false)

	metrics.RabbitMQEventsConsumed.WithLabelValues(rabbitmq.QueueQRWorker, "success").Inc()

	c.logger.Info("purchase processed successfully",
		"purchase_id", event.PurchaseID,
		"buyer_email", event.BuyerEmail,
		"qr_codes", len(qrImages),
	)
}
