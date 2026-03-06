package adapter

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/iponzi/entradasQR/internal/platform/metrics"
	"github.com/iponzi/entradasQR/internal/platform/rabbitmq"
	"github.com/iponzi/entradasQR/internal/validator"
	amqp "github.com/rabbitmq/amqp091-go"
)

// RabbitMQPublisher publishes validation events from the Validator Service
// to the RabbitMQ exchange for bidirectional reconciliation with the Ticket Service.
type RabbitMQPublisher struct {
	channel *amqp.Channel
}

// NewRabbitMQPublisher creates a new RabbitMQPublisher.
//
// Parameters:
//   - ch: An open AMQP channel for publishing messages.
//
// Returns:
//   - *RabbitMQPublisher: A pointer to the newly created publisher.
func NewRabbitMQPublisher(ch *amqp.Channel) *RabbitMQPublisher {
	return &RabbitMQPublisher{channel: ch}
}

// PublishTicketUsed publishes a ticket.used event to the RabbitMQ exchange
// so the Ticket Service can reconcile the ticket status in its own database.
//
// Parameters:
//   - ctx: The request context for cancellation and tracing.
//   - event: The TicketUsedEvent containing the ticket code and event ID.
//
// Returns:
//   - error: A wrapped error if marshaling or publishing fails; otherwise, nil.
func (p *RabbitMQPublisher) PublishTicketUsed(ctx context.Context, event validator.TicketUsedEvent) error {
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshaling ticket used event: %w", err)
	}

	err = p.channel.PublishWithContext(
		ctx,
		rabbitmq.ExchangeTicketEvents,
		rabbitmq.RoutingKeyUsed,
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		},
	)
	if err != nil {
		return fmt.Errorf("publishing ticket used event: %w", err)
	}

	metrics.RabbitMQEventsPublished.WithLabelValues(rabbitmq.RoutingKeyUsed).Inc()

	return nil
}
