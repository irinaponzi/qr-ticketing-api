package ticket

import (
	"errors"
	"time"
)

// Event represents a real-world event (concert, match, etc.) for which tickets are sold.
type Event struct {
	id         int
	attributes eventAttributes
}

type eventAttributes struct {
	name        string
	location    string
	date        time.Time
	capacity    int
	ticketPrice float64
	soldCount   int
	createdAt   time.Time
	updatedAt   time.Time
}

// NewEvent creates a new Event with the given parameters.
// It validates all invariants: positive ID, non-empty name and location,
// non-zero date, positive capacity, and non-negative ticket price.
//
// Parameters:
//   - id: The unique identifier for the event (must be positive).
//   - name: The display name of the event (must be non-empty).
//   - location: The venue or place where the event takes place (must be non-empty).
//   - date: The scheduled date and time (must be non-zero).
//   - capacity: The maximum number of tickets available (must be positive).
//   - ticketPrice: The price per ticket (must be positive).
//
// Returns:
//   - *Event: A pointer to the newly created Event.
//   - error: An error if any invariant is violated; otherwise, nil.
func NewEvent(id int, name, location string, date time.Time, capacity int, ticketPrice float64) (*Event, error) {
	if id <= 0 {
		return nil, errors.New("event ID must be positive")
	}

	if name == "" {
		return nil, errors.New("event name cannot be empty")
	}

	if location == "" {
		return nil, errors.New("event location cannot be empty")
	}

	if date.IsZero() {
		return nil, errors.New("event date cannot be zero")
	}

	if capacity <= 0 {
		return nil, errors.New("event capacity must be positive")
	}

	if ticketPrice <= 0 {
		return nil, errors.New("ticket price must be positive")
	}

	now := time.Now()

	return &Event{
		id: id,
		attributes: eventAttributes{
			name:        name,
			location:    location,
			date:        date,
			capacity:    capacity,
			ticketPrice: ticketPrice,
			soldCount:   0,
			createdAt:   now,
			updatedAt:   now,
		},
	}, nil
}

// NewEventFromRepository reconstructs an Event from persisted data.
// It bypasses validation since the data is assumed to be consistent.
//
// Parameters:
//   - id: The event identifier.
//   - name: The event name.
//   - location: The event venue.
//   - date: The scheduled date.
//   - capacity: Total ticket capacity.
//   - soldCount: Number of tickets already sold.
//   - createdAt: Original creation timestamp.
//   - updatedAt: Last modification timestamp.
//
// Returns:
//   - *Event: A pointer to the reconstructed Event.
func NewEventFromRepository(id int, name, location string, date time.Time, capacity int, ticketPrice float64, soldCount int, createdAt, updatedAt time.Time) *Event {
	return &Event{
		id: id,
		attributes: eventAttributes{
			name:        name,
			location:    location,
			date:        date,
			capacity:    capacity,
			ticketPrice: ticketPrice,
			soldCount:   soldCount,
			createdAt:   createdAt,
			updatedAt:   updatedAt,
		},
	}
}

// SetID assigns the database-generated identifier after persistence.
// It is intended to be called only by the repository layer after a successful insert.
func (e *Event) SetID(id int) { e.id = id }

// ID returns the unique identifier of the event.
func (e *Event) ID() int { return e.id }

// Name returns the display name of the event.
func (e *Event) Name() string { return e.attributes.name }

// Location returns the venue or place where the event takes place.
func (e *Event) Location() string { return e.attributes.location }

// Date returns the scheduled date and time of the event.
func (e *Event) Date() time.Time { return e.attributes.date }

// Capacity returns the maximum number of tickets that can be sold.
func (e *Event) Capacity() int { return e.attributes.capacity }

// TicketPrice returns the price per ticket for this event.
func (e *Event) TicketPrice() float64 { return e.attributes.ticketPrice }

// SoldCount returns the number of tickets already sold.
func (e *Event) SoldCount() int { return e.attributes.soldCount }

// CreatedAt returns the timestamp when the event was created.
func (e *Event) CreatedAt() time.Time { return e.attributes.createdAt }

// UpdatedAt returns the timestamp of the last modification.
func (e *Event) UpdatedAt() time.Time { return e.attributes.updatedAt }

// AvailableTickets returns the remaining ticket count (capacity - sold).
func (e *Event) AvailableTickets() int { return e.attributes.capacity - e.attributes.soldCount }

// HasAvailableTickets reports whether the event has at least the given quantity
// of tickets available for sale.
//
// Parameters:
//   - quantity: The number of tickets to check availability for.
//
// Returns:
//   - bool: True if at least quantity tickets are available; false otherwise.
func (e *Event) HasAvailableTickets(quantity int) bool {
	return e.AvailableTickets() >= quantity
}

// ReserveTickets reserves the given quantity of tickets for the event,
// incrementing the sold count and updating the modification timestamp.
//
// Parameters:
//   - quantity: The number of tickets to reserve (must be positive).
//
// Returns:
//   - error: An error if quantity is not positive or if there are not enough
//     tickets available; otherwise, nil.
func (e *Event) ReserveTickets(quantity int) error {
	if quantity <= 0 {
		return errors.New("quantity must be positive")
	}

	if !e.HasAvailableTickets(quantity) {
		return errors.New("not enough available tickets")
	}

	e.attributes.soldCount += quantity
	e.attributes.updatedAt = time.Now()

	return nil
}

// UpdateName changes the event name and updates the modification timestamp.
//
// Parameters:
//   - name: The new name for the event (must be non-empty).
//
// Returns:
//   - error: An error if the name is empty; otherwise, nil.
func (e *Event) UpdateName(name string) error {
	if name == "" {
		return errors.New("event name cannot be empty")
	}

	e.attributes.name = name
	e.attributes.updatedAt = time.Now()

	return nil
}
