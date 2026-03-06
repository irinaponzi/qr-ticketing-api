package rabbitmq

import (
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

// RabbitMQ topology constants define exchange, queue, and routing key names.
const (
	ExchangeTicketEvents = "ticket.events"
	QueueTicketCreated   = "validator.ticket.created"
	QueueTicketCancelled = "validator.ticket.cancelled"
	QueueQRWorker        = "qr-worker.purchase.completed"
	QueueTicketUsed      = "ticket.ticket.used"
	RoutingKeyCreated    = "ticket.created"
	RoutingKeyCancelled  = "ticket.cancelled"
	RoutingKeyPurchase   = "purchase.completed"
	RoutingKeyUsed       = "ticket.used"
)

// NewConnection establishes an AMQP connection to the given RabbitMQ URL.
func NewConnection(url string) (*amqp.Connection, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("connecting to rabbitmq: %w", err)
	}

	return conn, nil
}

// SetupTopology declares the exchange, queues, and bindings for ticket events.
func SetupTopology(ch *amqp.Channel) error {
	err := ch.ExchangeDeclare(
		ExchangeTicketEvents,
		"topic",
		true,  // durable
		false, // auto-deleted
		false, // internal
		false, // no-wait
		nil,
	)
	if err != nil {
		return fmt.Errorf("declaring exchange: %w", err)
	}

	if _, err := ch.QueueDeclare(QueueTicketCreated, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declaring queue %s: %w", QueueTicketCreated, err)
	}

	if _, err := ch.QueueDeclare(QueueTicketCancelled, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declaring queue %s: %w", QueueTicketCancelled, err)
	}

	if err := ch.QueueBind(QueueTicketCreated, RoutingKeyCreated, ExchangeTicketEvents, false, nil); err != nil {
		return fmt.Errorf("binding queue %s: %w", QueueTicketCreated, err)
	}

	if err := ch.QueueBind(QueueTicketCancelled, RoutingKeyCancelled, ExchangeTicketEvents, false, nil); err != nil {
		return fmt.Errorf("binding queue %s: %w", QueueTicketCancelled, err)
	}

	if _, err := ch.QueueDeclare(QueueQRWorker, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declaring queue %s: %w", QueueQRWorker, err)
	}

	if err := ch.QueueBind(QueueQRWorker, RoutingKeyPurchase, ExchangeTicketEvents, false, nil); err != nil {
		return fmt.Errorf("binding queue %s: %w", QueueQRWorker, err)
	}

	if _, err := ch.QueueDeclare(QueueTicketUsed, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declaring queue %s: %w", QueueTicketUsed, err)
	}

	if err := ch.QueueBind(QueueTicketUsed, RoutingKeyUsed, ExchangeTicketEvents, false, nil); err != nil {
		return fmt.Errorf("binding queue %s: %w", QueueTicketUsed, err)
	}

	return nil
}
