package ticket

import (
	"testing"
	"time"
)

func TestNewTicket_Success(t *testing.T) {
	ticket, err := NewTicket(1, 10, 100)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if ticket.ID() != 1 {
		t.Errorf("expected ID 1, got %d", ticket.ID())
	}

	if ticket.EventID() != 10 {
		t.Errorf("expected event ID 10, got %d", ticket.EventID())
	}

	if ticket.PurchaseID() != 100 {
		t.Errorf("expected purchase ID 100, got %d", ticket.PurchaseID())
	}

	if ticket.Status() != TicketStatusEmitted {
		t.Errorf("expected status 'emitted', got %q", ticket.Status())
	}

	if ticket.Code() == "" {
		t.Error("expected non-empty code (UUID)")
	}

	if ticket.UsedAt() != nil {
		t.Error("expected nil usedAt for new ticket")
	}

	if !ticket.IsValid() {
		t.Error("expected new ticket to be valid")
	}
}

func TestNewTicket_ValidationErrors(t *testing.T) {
	tests := []struct {
		name       string
		id         int
		eventID    int
		purchaseID int
		wantErr    string
	}{
		{
			name:       "negative ticket ID",
			id:         -1,
			eventID:    1,
			purchaseID: 1,
			wantErr:    "ticket ID cannot be negative",
		},
		{
			name:       "zero event ID",
			id:         1,
			eventID:    0,
			purchaseID: 1,
			wantErr:    "event ID must be positive",
		},
		{
			name:       "zero purchase ID",
			id:         1,
			eventID:    1,
			purchaseID: 0,
			wantErr:    "purchase ID must be positive",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ticket, err := NewTicket(tc.id, tc.eventID, tc.purchaseID)

			if ticket != nil {
				t.Error("expected nil ticket")
			}

			if err == nil {
				t.Fatal("expected error, got nil")
			}

			if err.Error() != tc.wantErr {
				t.Errorf("expected %q, got %q", tc.wantErr, err.Error())
			}
		})
	}
}

func TestTicket_MarkAsUsed(t *testing.T) {
	t.Run("mark emitted ticket as used", func(t *testing.T) {
		ticket := newTestTicket(t)

		err := ticket.MarkAsUsed()

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if ticket.Status() != TicketStatusUsed {
			t.Errorf("expected status 'used', got %q", ticket.Status())
		}

		if ticket.UsedAt() == nil {
			t.Error("expected usedAt to be set")
		}

		if ticket.IsValid() {
			t.Error("expected used ticket to not be valid")
		}
	})

	t.Run("mark already used ticket fails", func(t *testing.T) {
		ticket := newTestTicket(t)
		_ = ticket.MarkAsUsed()

		err := ticket.MarkAsUsed()

		if err == nil {
			t.Fatal("expected error, got nil")
		}

		if err.Error() != "ticket already used" {
			t.Errorf("expected 'ticket already used', got %q", err.Error())
		}
	})

	t.Run("mark cancelled ticket as used fails", func(t *testing.T) {
		ticket := newTestTicket(t)
		_ = ticket.Cancel()

		err := ticket.MarkAsUsed()

		if err == nil {
			t.Fatal("expected error, got nil")
		}

		if err.Error() != "ticket is cancelled" {
			t.Errorf("expected 'ticket is cancelled', got %q", err.Error())
		}
	})
}

func TestTicket_Cancel(t *testing.T) {
	t.Run("cancel emitted ticket", func(t *testing.T) {
		ticket := newTestTicket(t)

		err := ticket.Cancel()

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if ticket.Status() != TicketStatusCancelled {
			t.Errorf("expected status 'cancelled', got %q", ticket.Status())
		}

		if ticket.IsValid() {
			t.Error("expected cancelled ticket to not be valid")
		}
	})

	t.Run("cancel used ticket fails", func(t *testing.T) {
		ticket := newTestTicket(t)
		_ = ticket.MarkAsUsed()

		err := ticket.Cancel()

		if err == nil {
			t.Fatal("expected error, got nil")
		}

		if err.Error() != "cannot cancel a used ticket" {
			t.Errorf("expected 'cannot cancel a used ticket', got %q", err.Error())
		}
	})

	t.Run("cancel already cancelled ticket fails", func(t *testing.T) {
		ticket := newTestTicket(t)
		_ = ticket.Cancel()

		err := ticket.Cancel()

		if err == nil {
			t.Fatal("expected error, got nil")
		}

		if err.Error() != "ticket is already cancelled" {
			t.Errorf("expected 'ticket is already cancelled', got %q", err.Error())
		}
	})
}

func TestNewTicketFromRepository(t *testing.T) {
	now := time.Now()
	usedAt := now.Add(-time.Hour)

	ticket := NewTicketFromRepository(42, "abc-123", 10, 100, TicketStatusUsed, &usedAt, now.Add(-2*time.Hour), now)

	if ticket.ID() != 42 {
		t.Errorf("expected ID 42, got %d", ticket.ID())
	}

	if ticket.Code() != "abc-123" {
		t.Errorf("expected code 'abc-123', got %q", ticket.Code())
	}

	if ticket.Status() != TicketStatusUsed {
		t.Errorf("expected status 'used', got %q", ticket.Status())
	}

	if ticket.UsedAt() == nil || !ticket.UsedAt().Equal(usedAt) {
		t.Error("expected usedAt to match")
	}
}

func newTestTicket(t *testing.T) *Ticket {
	t.Helper()

	ticket, err := NewTicket(1, 10, 100)
	if err != nil {
		t.Fatalf("failed to create test ticket: %v", err)
	}

	return ticket
}
