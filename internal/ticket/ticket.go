package ticket

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// TicketStatus represents the lifecycle state of a ticket.
type TicketStatus string

const (
	// TicketStatusEmitted indicates the ticket has been issued and is ready to use.
	TicketStatusEmitted TicketStatus = "emitted"
	// TicketStatusUsed indicates the ticket has been validated at the venue.
	TicketStatusUsed TicketStatus = "used"
	// TicketStatusCancelled indicates the ticket has been revoked.
	TicketStatusCancelled TicketStatus = "cancelled"
)

// Ticket represents an entry pass for an event, identified by a unique QR code.
type Ticket struct {
	id         int
	attributes ticketAttributes
}

type ticketAttributes struct {
	code       string
	eventID    int
	purchaseID int
	status     TicketStatus
	usedAt     *time.Time
	createdAt  time.Time
	updatedAt  time.Time
}

// NewTicket creates a new Ticket in emitted status with an auto-generated UUID code.
// It validates that all IDs are positive.
//
// Parameters:
//   - id: The unique identifier for the ticket (must be positive).
//   - eventID: The ID of the event this ticket belongs to (must be positive).
//   - purchaseID: The ID of the purchase that originated this ticket (must be positive).
//
// Returns:
//   - *Ticket: A pointer to the newly created Ticket.
//   - error: An error if any invariant is violated; otherwise, nil.
func NewTicket(id, eventID, purchaseID int) (*Ticket, error) {
	if id < 0 {
		return nil, errors.New("ticket ID cannot be negative")
	}

	if eventID <= 0 {
		return nil, errors.New("event ID must be positive")
	}

	if purchaseID <= 0 {
		return nil, errors.New("purchase ID must be positive")
	}

	now := time.Now()

	return &Ticket{
		id: id,
		attributes: ticketAttributes{
			code:       uuid.NewString(),
			eventID:    eventID,
			purchaseID: purchaseID,
			status:     TicketStatusEmitted,
			createdAt:  now,
			updatedAt:  now,
		},
	}, nil
}

// NewTicketFromRepository reconstructs a Ticket from persisted data.
// It bypasses validation since the data is assumed to be consistent.
//
// Parameters:
//   - id: The ticket identifier.
//   - code: The UUID code for QR generation.
//   - eventID: The associated event ID.
//   - purchaseID: The associated purchase ID.
//   - status: The current lifecycle status.
//   - usedAt: The timestamp when the ticket was used, or nil.
//   - createdAt: Original creation timestamp.
//   - updatedAt: Last modification timestamp.
//
// Returns:
//   - *Ticket: A pointer to the reconstructed Ticket.
func NewTicketFromRepository(id int, code string, eventID, purchaseID int, status TicketStatus, usedAt *time.Time, createdAt, updatedAt time.Time) *Ticket {
	return &Ticket{
		id: id,
		attributes: ticketAttributes{
			code:       code,
			eventID:    eventID,
			purchaseID: purchaseID,
			status:     status,
			usedAt:     usedAt,
			createdAt:  createdAt,
			updatedAt:  updatedAt,
		},
	}
}

// SetID assigns the database-generated ID to the ticket after insertion.
// It can only be called once on a ticket with id = 0 (new entity).
func (t *Ticket) SetID(id int) error {
	if t.id != 0 {
		return errors.New("ticket ID already set")
	}

	if id <= 0 {
		return errors.New("ticket ID must be positive")
	}

	t.id = id

	return nil
}

// ID returns the unique identifier of the ticket.
func (t *Ticket) ID() int { return t.id }

// Code returns the UUID code used for QR generation and validation.
func (t *Ticket) Code() string { return t.attributes.code }

// EventID returns the ID of the event this ticket belongs to.
func (t *Ticket) EventID() int { return t.attributes.eventID }

// PurchaseID returns the ID of the purchase that originated this ticket.
func (t *Ticket) PurchaseID() int { return t.attributes.purchaseID }

// Status returns the current lifecycle status of the ticket.
func (t *Ticket) Status() TicketStatus { return t.attributes.status }

// UsedAt returns the timestamp when the ticket was used, or nil if not yet used.
func (t *Ticket) UsedAt() *time.Time { return t.attributes.usedAt }

// CreatedAt returns the timestamp when the ticket was created.
func (t *Ticket) CreatedAt() time.Time { return t.attributes.createdAt }

// UpdatedAt returns the timestamp of the last modification.
func (t *Ticket) UpdatedAt() time.Time { return t.attributes.updatedAt }

// IsValid reports whether the ticket can still be used (status is emitted).
func (t *Ticket) IsValid() bool {
	return t.attributes.status == TicketStatusEmitted
}

// MarkAsUsed transitions the ticket to used status, recording the current
// time as the usage timestamp.
//
// Returns:
//   - error: An error if the ticket is already used or cancelled; otherwise, nil.
func (t *Ticket) MarkAsUsed() error {
	if t.attributes.status == TicketStatusUsed {
		return errors.New("ticket already used")
	}

	if t.attributes.status == TicketStatusCancelled {
		return errors.New("ticket is cancelled")
	}

	now := time.Now()
	t.attributes.status = TicketStatusUsed
	t.attributes.usedAt = &now
	t.attributes.updatedAt = now

	return nil
}

// Cancel transitions the ticket to cancelled status.
// A used ticket cannot be cancelled.
//
// Returns:
//   - error: An error if the ticket is already used or already cancelled; otherwise, nil.
func (t *Ticket) Cancel() error {
	if t.attributes.status == TicketStatusUsed {
		return errors.New("cannot cancel a used ticket")
	}

	if t.attributes.status == TicketStatusCancelled {
		return errors.New("ticket is already cancelled")
	}

	t.attributes.status = TicketStatusCancelled
	t.attributes.updatedAt = time.Now()

	return nil
}
