package validator

import (
	"errors"
	"time"
)

// ValidTicketStatus represents the lifecycle state of a valid ticket in the validator service.
type ValidTicketStatus string

const (
	// ValidTicketStatusActive indicates the ticket is ready to be validated.
	ValidTicketStatusActive ValidTicketStatus = "active"
	// ValidTicketStatusUsed indicates the ticket has already been validated at the venue.
	ValidTicketStatusUsed ValidTicketStatus = "used"
	// ValidTicketStatusCancelled indicates the ticket has been revoked.
	ValidTicketStatusCancelled ValidTicketStatus = "cancelled"
)

// ValidTicket represents a ticket known to the validator service.
// This is a separate entity from the ticket domain; it is replicated via events.
type ValidTicket struct {
	id         int
	attributes validTicketAttributes
}

type validTicketAttributes struct {
	code      string
	eventID   int
	status    ValidTicketStatus
	usedAt    *time.Time
	syncedAt  time.Time
	updatedAt time.Time
}

// NewValidTicket creates a new ValidTicket in active status.
// An id of 0 is allowed for auto-generated IDs assigned by the database.
//
// Parameters:
//   - id: The identifier (0 for auto-assign, negative values are rejected).
//   - code: The UUID code of the original ticket (must be non-empty).
//   - eventID: The ID of the event this ticket belongs to (must be positive).
//
// Returns:
//   - *ValidTicket: A pointer to the newly created ValidTicket.
//   - error: An error if any invariant is violated; otherwise, nil.
func NewValidTicket(id int, code string, eventID int) (*ValidTicket, error) {
	if id < 0 {
		return nil, errors.New("valid ticket ID cannot be negative")
	}

	if code == "" {
		return nil, errors.New("ticket code cannot be empty")
	}

	if eventID <= 0 {
		return nil, errors.New("event ID must be positive")
	}

	now := time.Now()

	return &ValidTicket{
		id: id,
		attributes: validTicketAttributes{
			code:      code,
			eventID:   eventID,
			status:    ValidTicketStatusActive,
			syncedAt:  now,
			updatedAt: now,
		},
	}, nil
}

// NewValidTicketFromRepository reconstructs a ValidTicket from persisted data.
// It bypasses validation since the data is assumed to be consistent.
//
// Parameters:
//   - id: The valid ticket identifier.
//   - code: The UUID code of the original ticket.
//   - eventID: The associated event ID.
//   - status: The current lifecycle status.
//   - usedAt: The timestamp when the ticket was used, or nil.
//   - syncedAt: The timestamp when the ticket was synced from the ticket service.
//   - updatedAt: Last modification timestamp.
//
// Returns:
//   - *ValidTicket: A pointer to the reconstructed ValidTicket.
func NewValidTicketFromRepository(id int, code string, eventID int, status ValidTicketStatus, usedAt *time.Time, syncedAt, updatedAt time.Time) *ValidTicket {
	return &ValidTicket{
		id: id,
		attributes: validTicketAttributes{
			code:      code,
			eventID:   eventID,
			status:    status,
			usedAt:    usedAt,
			syncedAt:  syncedAt,
			updatedAt: updatedAt,
		},
	}
}

// ID returns the unique identifier of the valid ticket.
func (v *ValidTicket) ID() int { return v.id }

// Code returns the UUID code of the original ticket.
func (v *ValidTicket) Code() string { return v.attributes.code }

// EventID returns the ID of the event this ticket belongs to.
func (v *ValidTicket) EventID() int { return v.attributes.eventID }

// Status returns the current lifecycle status of the valid ticket.
func (v *ValidTicket) Status() ValidTicketStatus { return v.attributes.status }

// UsedAt returns the timestamp when the ticket was used, or nil if not yet used.
func (v *ValidTicket) UsedAt() *time.Time { return v.attributes.usedAt }

// SyncedAt returns the timestamp when the ticket was synced from the ticket service.
func (v *ValidTicket) SyncedAt() time.Time { return v.attributes.syncedAt }

// UpdatedAt returns the timestamp of the last modification.
func (v *ValidTicket) UpdatedAt() time.Time { return v.attributes.updatedAt }

// IsActive reports whether the ticket can still be validated (status is active).
func (v *ValidTicket) IsActive() bool {
	return v.attributes.status == ValidTicketStatusActive
}

// MarkAsUsed transitions the valid ticket to used status, recording the current
// time as the usage timestamp.
//
// Returns:
//   - error: An error if the ticket is already used or cancelled; otherwise, nil.
func (v *ValidTicket) MarkAsUsed() error {
	if v.attributes.status == ValidTicketStatusUsed {
		return errors.New("ticket already used")
	}

	if v.attributes.status == ValidTicketStatusCancelled {
		return errors.New("ticket is cancelled")
	}

	now := time.Now()
	v.attributes.status = ValidTicketStatusUsed
	v.attributes.usedAt = &now
	v.attributes.updatedAt = now

	return nil
}

// MarkAsCancelled transitions the valid ticket to cancelled status.
// A used ticket cannot be cancelled.
//
// Returns:
//   - error: An error if the ticket is already used or already cancelled; otherwise, nil.
func (v *ValidTicket) MarkAsCancelled() error {
	if v.attributes.status == ValidTicketStatusUsed {
		return errors.New("cannot cancel a used ticket")
	}

	if v.attributes.status == ValidTicketStatusCancelled {
		return errors.New("ticket is already cancelled")
	}

	v.attributes.status = ValidTicketStatusCancelled
	v.attributes.updatedAt = time.Now()

	return nil
}
