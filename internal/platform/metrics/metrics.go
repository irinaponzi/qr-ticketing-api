package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// HTTPRequestsTotal counts all HTTP requests by method, path, and status code.
	HTTPRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total number of HTTP requests",
	}, []string{"method", "path", "status"})

	// HTTPRequestDuration observes HTTP request durations by method and path.
	HTTPRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request duration in seconds",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "path"})

	// TicketsPurchasedTotal counts the total number of tickets purchased.
	TicketsPurchasedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "tickets_purchased_total",
		Help: "Total number of tickets purchased",
	})

	// EventsCreatedTotal counts the total number of events created.
	EventsCreatedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "events_created_total",
		Help: "Total number of events created",
	})

	// TicketsValidatedTotal counts ticket validation attempts by result.
	TicketsValidatedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "tickets_validated_total",
		Help: "Total number of ticket validations",
	}, []string{"result"})

	// RabbitMQEventsPublished counts events published to RabbitMQ by routing key.
	RabbitMQEventsPublished = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "rabbitmq_events_published_total",
		Help: "Total number of events published to RabbitMQ",
	}, []string{"routing_key"})

	// RabbitMQEventsConsumed counts events consumed from RabbitMQ by queue and status.
	RabbitMQEventsConsumed = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "rabbitmq_events_consumed_total",
		Help: "Total number of events consumed from RabbitMQ",
	}, []string{"queue", "status"})
)
