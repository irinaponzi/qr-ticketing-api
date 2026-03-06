package adapter

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/iponzi/entradasQR/internal/platform/metrics"
	"github.com/iponzi/entradasQR/internal/platform/rabbitmq"
	"github.com/iponzi/entradasQR/internal/ticket"
	amqp "github.com/rabbitmq/amqp091-go"
)

// RabbitMQPublisher implements TicketEventPublisher by publishing ticket lifecycle
// events to a RabbitMQ exchange using topic routing.
type RabbitMQPublisher struct {
	channel *amqp.Channel
}

// NewRabbitMQPublisher creates a new RabbitMQPublisher bound to the given AMQP channel.
//
// Parameters:
//   - ch: An open AMQP channel for publishing messages.
//
// Returns:
//   - *RabbitMQPublisher: A pointer to the newly created publisher.
func NewRabbitMQPublisher(ch *amqp.Channel) *RabbitMQPublisher {
	return &RabbitMQPublisher{channel: ch}
}

// PublishTicketCreated publishes a ticket.created event to the RabbitMQ exchange.
// The event is serialized as JSON and routed with the ticket.created routing key.
//
// Parameters:
//   - ctx: The request context for cancellation and tracing.
//   - event: The ticket creation event payload.
//
// Returns:
//   - error: A wrapped error if marshaling or publishing fails; otherwise, nil.
func (p *RabbitMQPublisher) PublishTicketCreated(ctx context.Context, event ticket.TicketCreatedEvent) error {
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshaling ticket created event: %w", err)
	}

	if err := p.channel.PublishWithContext(ctx,
		rabbitmq.ExchangeTicketEvents,
		rabbitmq.RoutingKeyCreated,
		false, false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		},
	); err != nil {
		return err
	}

	metrics.RabbitMQEventsPublished.WithLabelValues(rabbitmq.RoutingKeyCreated).Inc()

	return nil
}

// PublishTicketCancelled publishes a ticket.cancelled event to the RabbitMQ exchange.
// The event is serialized as JSON and routed with the ticket.cancelled routing key.
//
// Parameters:
//   - ctx: The request context for cancellation and tracing.
//   - event: The ticket cancellation event payload.
//
// Returns:
//   - error: A wrapped error if marshaling or publishing fails; otherwise, nil.
func (p *RabbitMQPublisher) PublishTicketCancelled(ctx context.Context, event ticket.TicketCancelledEvent) error {
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshaling ticket cancelled event: %w", err)
	}

	if err := p.channel.PublishWithContext(ctx,
		rabbitmq.ExchangeTicketEvents,
		rabbitmq.RoutingKeyCancelled,
		false, false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		},
	); err != nil {
		return err
	}

	metrics.RabbitMQEventsPublished.WithLabelValues(rabbitmq.RoutingKeyCancelled).Inc()

	return nil
}

// PublishPurchaseCompleted publishes a purchase.completed event to the RabbitMQ exchange.
// The QR worker consumes this event to generate QR codes and send the confirmation email.
//
// Parameters:
//   - ctx: The request context for cancellation and tracing.
//   - event: The purchase completed event payload containing buyer and ticket info.
//
// Returns:
//   - error: A wrapped error if marshaling or publishing fails; otherwise, nil.
func (p *RabbitMQPublisher) PublishPurchaseCompleted(ctx context.Context, event ticket.PurchaseCompletedEvent) error {
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshaling purchase completed event: %w", err)
	}

	if err := p.channel.PublishWithContext(ctx,
		rabbitmq.ExchangeTicketEvents,
		rabbitmq.RoutingKeyPurchase,
		false, false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		},
	); err != nil {
		return err
	}

	metrics.RabbitMQEventsPublished.WithLabelValues(rabbitmq.RoutingKeyPurchase).Inc()

	return nil
}
