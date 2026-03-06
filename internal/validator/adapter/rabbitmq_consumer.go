package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/iponzi/entradasQR/internal/platform/metrics"
	"github.com/iponzi/entradasQR/internal/platform/rabbitmq"
	"github.com/iponzi/entradasQR/internal/validator"
	amqp "github.com/rabbitmq/amqp091-go"
)

type ticketCreatedMessage struct {
	TicketID   int    `json:"TicketID"`
	TicketCode string `json:"TicketCode"`
	EventID    int    `json:"EventID"`
}

type ticketCancelledMessage struct {
	TicketID   int    `json:"TicketID"`
	TicketCode string `json:"TicketCode"`
	EventID    int    `json:"EventID"`
}

// RabbitMQConsumer listens for ticket events and syncs them to the validator DB.
type RabbitMQConsumer struct {
	channel *amqp.Channel
	service *validator.ValidatorService
	logger  *slog.Logger
}

// NewRabbitMQConsumer creates a new RabbitMQConsumer that listens for ticket
// lifecycle events and delegates processing to the ValidatorService.
//
// Parameters:
//   - ch: An open AMQP channel for consuming messages.
//   - service: The validator domain service for syncing ticket state.
//   - logger: Structured logger for observability.
//
// Returns:
//   - *RabbitMQConsumer: A pointer to the newly created consumer.
func NewRabbitMQConsumer(ch *amqp.Channel, service *validator.ValidatorService, logger *slog.Logger) *RabbitMQConsumer {
	return &RabbitMQConsumer{
		channel: ch,
		service: service,
		logger:  logger,
	}
}

// StartConsuming begins listening on the ticket.created and ticket.cancelled
// queues in separate goroutines. It processes messages idempotently and
// acknowledges them on success.
//
// Parameters:
//   - ctx: The context for cancellation; when cancelled, consumers stop.
//
// Returns:
//   - error: A wrapped error if registering consumers fails; otherwise, nil.
func (c *RabbitMQConsumer) StartConsuming(ctx context.Context) error {
	if err := c.consumeCreated(ctx); err != nil {
		return fmt.Errorf("starting created consumer: %w", err)
	}

	if err := c.consumeCancelled(ctx); err != nil {
		return fmt.Errorf("starting cancelled consumer: %w", err)
	}

	return nil
}

func (c *RabbitMQConsumer) consumeCreated(ctx context.Context) error {
	msgs, err := c.channel.Consume(rabbitmq.QueueTicketCreated, "", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("consuming from %s: %w", rabbitmq.QueueTicketCreated, err)
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-msgs:
				if !ok {
					return
				}

				c.handleCreated(ctx, msg)
			}
		}
	}()

	return nil
}

func (c *RabbitMQConsumer) consumeCancelled(ctx context.Context) error {
	msgs, err := c.channel.Consume(rabbitmq.QueueTicketCancelled, "", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("consuming from %s: %w", rabbitmq.QueueTicketCancelled, err)
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-msgs:
				if !ok {
					return
				}

				c.handleCancelled(ctx, msg)
			}
		}
	}()

	return nil
}

func (c *RabbitMQConsumer) handleCreated(ctx context.Context, msg amqp.Delivery) {
	var event ticketCreatedMessage
	if err := json.Unmarshal(msg.Body, &event); err != nil {
		c.logger.ErrorContext(ctx, "failed to unmarshal ticket.created", "error", err)
		msg.Nack(false, false)

		return
	}

	if err := c.service.SyncTicketCreated(ctx, event.TicketCode, event.EventID); err != nil {
		c.logger.ErrorContext(ctx, "failed to sync ticket.created", "code", event.TicketCode, "error", err)
		msg.Nack(false, true)

		return
	}

	msg.Ack(false)
	metrics.RabbitMQEventsConsumed.WithLabelValues(rabbitmq.QueueTicketCreated, "ok").Inc()
	c.logger.InfoContext(ctx, "synced ticket.created", "code", event.TicketCode)
}

func (c *RabbitMQConsumer) handleCancelled(ctx context.Context, msg amqp.Delivery) {
	var event ticketCancelledMessage
	if err := json.Unmarshal(msg.Body, &event); err != nil {
		c.logger.ErrorContext(ctx, "failed to unmarshal ticket.cancelled", "error", err)
		msg.Nack(false, false)

		return
	}

	if err := c.service.SyncTicketCancelled(ctx, event.TicketCode); err != nil {
		c.logger.ErrorContext(ctx, "failed to sync ticket.cancelled", "code", event.TicketCode, "error", err)
		msg.Nack(false, true)

		return
	}

	msg.Ack(false)
	metrics.RabbitMQEventsConsumed.WithLabelValues(rabbitmq.QueueTicketCancelled, "ok").Inc()
	c.logger.InfoContext(ctx, "synced ticket.cancelled", "code", event.TicketCode)
}
