package storage

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/iponzi/entradasQR/internal/ticket"
)

func TestMySQLEventRepository_Get_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	now := time.Now().Truncate(time.Second)
	rows := sqlmock.NewRows([]string{"id", "name", "location", "date", "capacity", "ticket_price", "sold_count", "created_at", "updated_at"}).
		AddRow(1, "Concert", "Stadium", now, 500, 150.0, 10, now, now)

	mock.ExpectQuery("SELECT id, name, location, date, capacity, ticket_price, sold_count, created_at, updated_at FROM events WHERE id = \\?").
		WithArgs(1).
		WillReturnRows(rows)

	repo := NewMySQLEventRepository(db)
	event, err := repo.Get(context.Background(), 1)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if event == nil {
		t.Fatal("expected event, got nil")
	}

	if event.Name() != "Concert" {
		t.Errorf("expected name 'Concert', got %q", event.Name())
	}

	if event.Capacity() != 500 {
		t.Errorf("expected capacity 500, got %d", event.Capacity())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestMySQLEventRepository_Get_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id", "name", "location", "date", "capacity", "ticket_price", "sold_count", "created_at", "updated_at"})

	mock.ExpectQuery("SELECT id, name, location, date, capacity, ticket_price, sold_count, created_at, updated_at FROM events WHERE id = \\?").
		WithArgs(999).
		WillReturnRows(rows)

	repo := NewMySQLEventRepository(db)
	event, err := repo.Get(context.Background(), 999)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if event != nil {
		t.Errorf("expected nil, got event with ID %d", event.ID())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestMySQLEventRepository_Get_DBError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery("SELECT id, name, location, date, capacity, ticket_price, sold_count, created_at, updated_at FROM events WHERE id = \\?").
		WithArgs(1).
		WillReturnError(errors.New("connection refused"))

	repo := NewMySQLEventRepository(db)
	_, err = repo.Get(context.Background(), 1)

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestMySQLEventRepository_Add_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	date := time.Date(2026, 6, 15, 20, 0, 0, 0, time.UTC)
	event, _ := ticket.NewEvent(1, "Concert", "Stadium", date, 500, 150.0)

	mock.ExpectExec("INSERT INTO events").
		WithArgs(event.Name(), event.Location(), event.Date(), event.Capacity(), event.TicketPrice(), event.SoldCount(), event.CreatedAt(), event.UpdatedAt()).
		WillReturnResult(sqlmock.NewResult(42, 1))

	repo := NewMySQLEventRepository(db)
	err = repo.Add(context.Background(), event)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if event.ID() != 42 {
		t.Errorf("expected event ID 42 after Add, got %d", event.ID())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestMySQLEventRepository_Add_DBError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	date := time.Date(2026, 6, 15, 20, 0, 0, 0, time.UTC)
	event, _ := ticket.NewEvent(1, "Concert", "Stadium", date, 500, 150.0)

	mock.ExpectExec("INSERT INTO events").
		WithArgs(event.Name(), event.Location(), event.Date(), event.Capacity(), event.TicketPrice(), event.SoldCount(), event.CreatedAt(), event.UpdatedAt()).
		WillReturnError(errors.New("duplicate entry"))

	repo := NewMySQLEventRepository(db)
	err = repo.Add(context.Background(), event)

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestMySQLEventRepository_Update_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	date := time.Date(2026, 6, 15, 20, 0, 0, 0, time.UTC)
	event, _ := ticket.NewEvent(1, "Concert", "Stadium", date, 500, 150.0)

	mock.ExpectExec("UPDATE events SET").
		WithArgs(event.Name(), event.Location(), event.Date(), event.Capacity(), event.TicketPrice(), event.SoldCount(), event.UpdatedAt(), event.ID()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	repo := NewMySQLEventRepository(db)
	err = repo.Update(context.Background(), event)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestMySQLEventRepository_Update_DBError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	date := time.Date(2026, 6, 15, 20, 0, 0, 0, time.UTC)
	event, _ := ticket.NewEvent(1, "Concert", "Stadium", date, 500, 150.0)

	mock.ExpectExec("UPDATE events SET").
		WithArgs(event.Name(), event.Location(), event.Date(), event.Capacity(), event.TicketPrice(), event.SoldCount(), event.UpdatedAt(), event.ID()).
		WillReturnError(errors.New("deadlock"))

	repo := NewMySQLEventRepository(db)
	err = repo.Update(context.Background(), event)

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
