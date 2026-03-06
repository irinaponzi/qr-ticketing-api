package validator

import (
	"testing"
	"time"
)

func TestNewValidTicket_Success(t *testing.T) {
	vt, err := NewValidTicket(1, "abc-123", 10)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if vt.ID() != 1 {
		t.Errorf("expected ID 1, got %d", vt.ID())
	}

	if vt.Code() != "abc-123" {
		t.Errorf("expected code 'abc-123', got %q", vt.Code())
	}

	if vt.EventID() != 10 {
		t.Errorf("expected event ID 10, got %d", vt.EventID())
	}

	if vt.Status() != ValidTicketStatusActive {
		t.Errorf("expected status 'active', got %q", vt.Status())
	}

	if !vt.IsActive() {
		t.Error("expected ticket to be active")
	}

	if vt.UsedAt() != nil {
		t.Error("expected nil usedAt for new ticket")
	}
}

func TestNewValidTicket_ValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		id      int
		code    string
		eventID int
		wantErr string
	}{
		{
			name:    "negative ID",
			id:      -1,
			code:    "abc-123",
			eventID: 10,
			wantErr: "valid ticket ID cannot be negative",
		},
		{
			name:    "empty code",
			id:      1,
			code:    "",
			eventID: 10,
			wantErr: "ticket code cannot be empty",
		},
		{
			name:    "zero event ID",
			id:      1,
			code:    "abc-123",
			eventID: 0,
			wantErr: "event ID must be positive",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			vt, err := NewValidTicket(tc.id, tc.code, tc.eventID)

			if vt != nil {
				t.Error("expected nil valid ticket")
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

func TestValidTicket_MarkAsUsed(t *testing.T) {
	t.Run("mark active ticket as used", func(t *testing.T) {
		vt, _ := NewValidTicket(1, "abc-123", 10)

		err := vt.MarkAsUsed()

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if vt.Status() != ValidTicketStatusUsed {
			t.Errorf("expected status 'used', got %q", vt.Status())
		}

		if vt.UsedAt() == nil {
			t.Error("expected usedAt to be set")
		}

		if vt.IsActive() {
			t.Error("expected used ticket to not be active")
		}
	})

	t.Run("mark already used fails", func(t *testing.T) {
		vt, _ := NewValidTicket(1, "abc-123", 10)
		_ = vt.MarkAsUsed()

		err := vt.MarkAsUsed()

		if err == nil {
			t.Fatal("expected error, got nil")
		}

		if err.Error() != "ticket already used" {
			t.Errorf("expected 'ticket already used', got %q", err.Error())
		}
	})

	t.Run("mark cancelled ticket as used fails", func(t *testing.T) {
		vt, _ := NewValidTicket(1, "abc-123", 10)
		_ = vt.MarkAsCancelled()

		err := vt.MarkAsUsed()

		if err == nil {
			t.Fatal("expected error, got nil")
		}

		if err.Error() != "ticket is cancelled" {
			t.Errorf("expected 'ticket is cancelled', got %q", err.Error())
		}
	})
}

func TestValidTicket_MarkAsCancelled(t *testing.T) {
	t.Run("cancel active ticket", func(t *testing.T) {
		vt, _ := NewValidTicket(1, "abc-123", 10)

		err := vt.MarkAsCancelled()

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if vt.Status() != ValidTicketStatusCancelled {
			t.Errorf("expected status 'cancelled', got %q", vt.Status())
		}

		if vt.IsActive() {
			t.Error("expected cancelled ticket to not be active")
		}
	})

	t.Run("cancel used ticket fails", func(t *testing.T) {
		vt, _ := NewValidTicket(1, "abc-123", 10)
		_ = vt.MarkAsUsed()

		err := vt.MarkAsCancelled()

		if err == nil {
			t.Fatal("expected error, got nil")
		}

		if err.Error() != "cannot cancel a used ticket" {
			t.Errorf("expected 'cannot cancel a used ticket', got %q", err.Error())
		}
	})

	t.Run("cancel already cancelled fails", func(t *testing.T) {
		vt, _ := NewValidTicket(1, "abc-123", 10)
		_ = vt.MarkAsCancelled()

		err := vt.MarkAsCancelled()

		if err == nil {
			t.Fatal("expected error, got nil")
		}

		if err.Error() != "ticket is already cancelled" {
			t.Errorf("expected 'ticket is already cancelled', got %q", err.Error())
		}
	})
}

func TestNewValidTicketFromRepository(t *testing.T) {
	now := time.Now()
	usedAt := now.Add(-time.Hour)

	vt := NewValidTicketFromRepository(42, "xyz-789", 10, ValidTicketStatusUsed, &usedAt, now.Add(-2*time.Hour), now)

	if vt.ID() != 42 {
		t.Errorf("expected ID 42, got %d", vt.ID())
	}

	if vt.Code() != "xyz-789" {
		t.Errorf("expected code 'xyz-789', got %q", vt.Code())
	}

	if vt.Status() != ValidTicketStatusUsed {
		t.Errorf("expected status 'used', got %q", vt.Status())
	}

	if vt.UsedAt() == nil || !vt.UsedAt().Equal(usedAt) {
		t.Error("expected usedAt to match")
	}
}
