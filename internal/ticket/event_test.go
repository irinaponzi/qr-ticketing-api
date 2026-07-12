package ticket

import (
	"testing"
	"time"
)

func TestNewEvent_Success(t *testing.T) {
	date := time.Date(2026, 6, 15, 20, 0, 0, 0, time.UTC)

	event, err := NewEvent(1, "Rock Concert", "Stadium", date, 1000, 150.0)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if event.ID() != 1 {
		t.Errorf("expected ID 1, got %d", event.ID())
	}

	if event.Name() != "Rock Concert" {
		t.Errorf("expected name 'Rock Concert', got '%s'", event.Name())
	}

	if event.Location() != "Stadium" {
		t.Errorf("expected location 'Stadium', got '%s'", event.Location())
	}

	if event.Capacity() != 1000 {
		t.Errorf("expected capacity 1000, got %d", event.Capacity())
	}

	if event.SoldCount() != 0 {
		t.Errorf("expected sold count 0, got %d", event.SoldCount())
	}

	if event.AvailableTickets() != 1000 {
		t.Errorf("expected available 1000, got %d", event.AvailableTickets())
	}

	if event.TicketPrice() != 150.0 {
		t.Errorf("expected ticket price 150.0, got %f", event.TicketPrice())
	}
}

func TestNewEvent_ValidationErrors(t *testing.T) {
	validDate := time.Date(2026, 6, 15, 20, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		id       int
		evName   string
		location string
		date     time.Time
		capacity int
		wantErr  string
	}{
		{
			name:     "negative ID",
			id:       -1,
			evName:   "Concert",
			location: "Stadium",
			date:     validDate,
			capacity: 100,
			wantErr:  "event ID cannot be negative",
		},
		{
			name:     "empty name",
			id:       1,
			evName:   "",
			location: "Stadium",
			date:     validDate,
			capacity: 100,
			wantErr:  "event name cannot be empty",
		},
		{
			name:     "empty location",
			id:       1,
			evName:   "Concert",
			location: "",
			date:     validDate,
			capacity: 100,
			wantErr:  "event location cannot be empty",
		},
		{
			name:     "zero date",
			id:       1,
			evName:   "Concert",
			location: "Stadium",
			date:     time.Time{},
			capacity: 100,
			wantErr:  "event date cannot be zero",
		},
		{
			name:     "zero capacity",
			id:       1,
			evName:   "Concert",
			location: "Stadium",
			date:     validDate,
			capacity: 0,
			wantErr:  "event capacity must be positive",
		},
		{
			name:     "negative capacity",
			id:       1,
			evName:   "Concert",
			location: "Stadium",
			date:     validDate,
			capacity: -5,
			wantErr:  "event capacity must be positive",
		},
		{
			name:     "zero ticket price",
			id:       1,
			evName:   "Concert",
			location: "Stadium",
			date:     validDate,
			capacity: 100,
			wantErr:  "ticket price must be positive",
		},
		{
			name:     "negative ticket price",
			id:       1,
			evName:   "Concert",
			location: "Stadium",
			date:     validDate,
			capacity: 100,
			wantErr:  "ticket price must be positive",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			price := 100.0
			if tc.wantErr == "ticket price must be positive" {
				if tc.name == "zero ticket price" {
					price = 0
				} else {
					price = -10.0
				}
			}

			event, err := NewEvent(tc.id, tc.evName, tc.location, tc.date, tc.capacity, price)

			if event != nil {
				t.Error("expected nil event on validation error")
			}

			if err == nil {
				t.Fatal("expected error, got nil")
			}

			if err.Error() != tc.wantErr {
				t.Errorf("expected error %q, got %q", tc.wantErr, err.Error())
			}
		})
	}
}

func TestEvent_ReserveTickets(t *testing.T) {
	t.Run("successful reservation", func(t *testing.T) {
		event := newTestEvent(t, 100)

		err := event.ReserveTickets(10)

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if event.SoldCount() != 10 {
			t.Errorf("expected sold count 10, got %d", event.SoldCount())
		}

		if event.AvailableTickets() != 90 {
			t.Errorf("expected available 90, got %d", event.AvailableTickets())
		}
	})

	t.Run("reserve all tickets", func(t *testing.T) {
		event := newTestEvent(t, 5)

		err := event.ReserveTickets(5)

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if event.AvailableTickets() != 0 {
			t.Errorf("expected available 0, got %d", event.AvailableTickets())
		}
	})

	t.Run("not enough tickets", func(t *testing.T) {
		event := newTestEvent(t, 5)

		err := event.ReserveTickets(10)

		if err == nil {
			t.Fatal("expected error, got nil")
		}

		if err.Error() != "not enough available tickets" {
			t.Errorf("expected 'not enough available tickets', got %q", err.Error())
		}
	})

	t.Run("zero quantity", func(t *testing.T) {
		event := newTestEvent(t, 100)

		err := event.ReserveTickets(0)

		if err == nil {
			t.Fatal("expected error, got nil")
		}

		if err.Error() != "quantity must be positive" {
			t.Errorf("expected 'quantity must be positive', got %q", err.Error())
		}
	})

	t.Run("negative quantity", func(t *testing.T) {
		event := newTestEvent(t, 100)

		err := event.ReserveTickets(-1)

		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestEvent_HasAvailableTickets(t *testing.T) {
	event := newTestEvent(t, 10)
	_ = event.ReserveTickets(8)

	if !event.HasAvailableTickets(2) {
		t.Error("expected 2 tickets to be available")
	}

	if event.HasAvailableTickets(3) {
		t.Error("expected 3 tickets to NOT be available")
	}
}

func TestEvent_UpdateName(t *testing.T) {
	event := newTestEvent(t, 100)

	err := event.UpdateName("Jazz Night")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if event.Name() != "Jazz Night" {
		t.Errorf("expected 'Jazz Night', got %q", event.Name())
	}

	err = event.UpdateName("")
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestEvent_SetID(t *testing.T) {
	event := newTestEvent(t, 100)

	if event.ID() != 0 {
		t.Fatalf("expected initial ID 0, got %d", event.ID())
	}

	event.SetID(42)

	if event.ID() != 42 {
		t.Errorf("expected ID 42 after SetID, got %d", event.ID())
	}
}

func newTestEvent(t *testing.T, capacity int) *Event {
	t.Helper()

	date := time.Date(2026, 6, 15, 20, 0, 0, 0, time.UTC)

	event, err := NewEvent(0, "Test Event", "Test Venue", date, capacity, 100.0)
	if err != nil {
		t.Fatalf("failed to create test event: %v", err)
	}

	return event
}
