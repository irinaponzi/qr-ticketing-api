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

type ticketUsedMessage struct {
	TicketCode string `json:"TicketCode"`
	EventID    int    `json:"EventID"`
}

// TicketUsedConsumer listens for ticket.used events from the Validator Service
// and reconciles ticket state in the Ticket Service database.
type TicketUsedConsumer struct {
	channel *amqp.Channel
	service *ticket.TicketService
	logger  *slog.Logger
}

// NewTicketUsedConsumer creates a new TicketUsedConsumer.
//
// Parameters:
//   - ch: An open AMQP channel for consuming messages.
//   - service: The ticket domain service for marking tickets as used.
//   - logger: Structured logger for observability.
//
// Returns:
//   - *TicketUsedConsumer: A pointer to the newly created consumer.
func NewTicketUsedConsumer(ch *amqp.Channel, service *ticket.TicketService, logger *slog.Logger) *TicketUsedConsumer {
	return &TicketUsedConsumer{
		channel: ch,
		service: service,
		logger:  logger,
	}
}

// StartConsuming begins listening on the ticket.ticket.used queue.
// It processes messages idempotently, marking tickets as used in the local DB.
//
// Parameters:
//   - ctx: The context for cancellation; when cancelled, the consumer stops.
//
// Returns:
//   - error: A wrapped error if registering the consumer fails; otherwise, nil.
func (c *TicketUsedConsumer) StartConsuming(ctx context.Context) error {
	msgs, err := c.channel.Consume(rabbitmq.QueueTicketUsed, "", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("consuming from %s: %w", rabbitmq.QueueTicketUsed, err)
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

				c.handleMessage(ctx, msg)
			}
		}
	}()

	return nil
}

func (c *TicketUsedConsumer) handleMessage(ctx context.Context, msg amqp.Delivery) {
	var event ticketUsedMessage
	if err := json.Unmarshal(msg.Body, &event); err != nil {
		c.logger.ErrorContext(ctx, "failed to unmarshal ticket.used", "error", err)
		msg.Nack(false, false)

		return
	}

	if err := c.service.SyncTicketUsed(ctx, event.TicketCode); err != nil {
		c.logger.ErrorContext(ctx, "failed to sync ticket.used", "code", event.TicketCode, "error", err)
		msg.Nack(false, true)

		return
	}

	msg.Ack(false)
	metrics.RabbitMQEventsConsumed.WithLabelValues(rabbitmq.QueueTicketUsed, "ok").Inc()
	c.logger.InfoContext(ctx, "synced ticket.used", "code", event.TicketCode)
}
