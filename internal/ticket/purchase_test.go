package ticket

import (
	"testing"
)

func TestNewPurchase_Success(t *testing.T) {
	purchase, err := NewPurchase(1, "user@example.com", 10, 3, 300.0)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if purchase.ID() != 1 {
		t.Errorf("expected ID 1, got %d", purchase.ID())
	}

	if purchase.BuyerEmail() != "user@example.com" {
		t.Errorf("expected email 'user@example.com', got %q", purchase.BuyerEmail())
	}

	if purchase.EventID() != 10 {
		t.Errorf("expected event ID 10, got %d", purchase.EventID())
	}

	if purchase.Quantity() != 3 {
		t.Errorf("expected quantity 3, got %d", purchase.Quantity())
	}

	if purchase.TotalPrice() != 300.0 {
		t.Errorf("expected total price 300.0, got %f", purchase.TotalPrice())
	}

	if len(purchase.Tickets()) != 0 {
		t.Errorf("expected 0 tickets initially, got %d", len(purchase.Tickets()))
	}
}

func TestNewPurchase_ValidationErrors(t *testing.T) {
	tests := []struct {
		name       string
		id         int
		email      string
		eventID    int
		quantity   int
		totalPrice float64
		wantErr    string
	}{
		{
			name:       "negative ID",
			id:         -1,
			email:      "user@example.com",
			eventID:    1,
			quantity:   1,
			totalPrice: 100.0,
			wantErr:    "purchase ID cannot be negative",
		},
		{
			name:       "empty email",
			id:         1,
			email:      "",
			eventID:    1,
			quantity:   1,
			totalPrice: 100.0,
			wantErr:    "buyer email cannot be empty",
		},
		{
			name:       "zero event ID",
			id:         1,
			email:      "user@example.com",
			eventID:    0,
			quantity:   1,
			totalPrice: 100.0,
			wantErr:    "event ID must be positive",
		},
		{
			name:       "zero quantity",
			id:         1,
			email:      "user@example.com",
			eventID:    1,
			quantity:   0,
			totalPrice: 100.0,
			wantErr:    "quantity must be positive",
		},
		{
			name:       "negative total price",
			id:         1,
			email:      "user@example.com",
			eventID:    1,
			quantity:   1,
			totalPrice: -10.0,
			wantErr:    "total price cannot be negative",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			purchase, err := NewPurchase(tc.id, tc.email, tc.eventID, tc.quantity, tc.totalPrice)

			if purchase != nil {
				t.Error("expected nil purchase")
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

func TestPurchase_AddTicket(t *testing.T) {
	t.Run("add ticket successfully", func(t *testing.T) {
		purchase, _ := NewPurchase(1, "user@example.com", 10, 2, 200.0)
		ticket, _ := NewTicket(1, 10, 1)

		err := purchase.AddTicket(ticket)

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if len(purchase.Tickets()) != 1 {
			t.Errorf("expected 1 ticket, got %d", len(purchase.Tickets()))
		}
	})

	t.Run("add nil ticket fails", func(t *testing.T) {
		purchase, _ := NewPurchase(1, "user@example.com", 10, 2, 200.0)

		err := purchase.AddTicket(nil)

		if err == nil {
			t.Fatal("expected error, got nil")
		}

		if err.Error() != "ticket cannot be nil" {
			t.Errorf("expected 'ticket cannot be nil', got %q", err.Error())
		}
	})

	t.Run("exceeding quantity fails", func(t *testing.T) {
		purchase, _ := NewPurchase(1, "user@example.com", 10, 1, 100.0)
		ticket1, _ := NewTicket(1, 10, 1)
		ticket2, _ := NewTicket(2, 10, 1)

		_ = purchase.AddTicket(ticket1)
		err := purchase.AddTicket(ticket2)

		if err == nil {
			t.Fatal("expected error, got nil")
		}

		if err.Error() != "purchase already has all tickets assigned" {
			t.Errorf("expected 'purchase already has all tickets assigned', got %q", err.Error())
		}
	})
}

func TestPurchase_TicketCodes(t *testing.T) {
	purchase, _ := NewPurchase(1, "user@example.com", 10, 2, 200.0)
	t1, _ := NewTicket(1, 10, 1)
	t2, _ := NewTicket(2, 10, 1)

	_ = purchase.AddTicket(t1)
	_ = purchase.AddTicket(t2)

	codes := purchase.TicketCodes()

	if len(codes) != 2 {
		t.Fatalf("expected 2 codes, got %d", len(codes))
	}

	if codes[0] == "" || codes[1] == "" {
		t.Error("expected non-empty ticket codes")
	}

	if codes[0] == codes[1] {
		t.Error("expected unique codes for each ticket")
	}
}
