package storage

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/iponzi/entradasQR/internal/ticket"
)

func TestMySQLTicketRepository_Get_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	now := time.Now().Truncate(time.Second)
	rows := sqlmock.NewRows([]string{"id", "code", "event_id", "purchase_id", "status", "used_at", "created_at", "updated_at"}).
		AddRow(1, "abc-123", 10, 100, "emitted", nil, now, now)

	mock.ExpectQuery("SELECT id, code, event_id, purchase_id, status, used_at, created_at, updated_at FROM tickets WHERE id = \\?").
		WithArgs(1).
		WillReturnRows(rows)

	repo := NewMySQLTicketRepository(db)
	tk, err := repo.Get(context.Background(), 1)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if tk == nil {
		t.Fatal("expected ticket, got nil")
	}

	if tk.Code() != "abc-123" {
		t.Errorf("expected code 'abc-123', got %q", tk.Code())
	}

	if tk.Status() != ticket.TicketStatusEmitted {
		t.Errorf("expected status emitted, got %q", tk.Status())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestMySQLTicketRepository_Get_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id", "code", "event_id", "purchase_id", "status", "used_at", "created_at", "updated_at"})

	mock.ExpectQuery("SELECT id, code, event_id, purchase_id, status, used_at, created_at, updated_at FROM tickets WHERE id = \\?").
		WithArgs(999).
		WillReturnRows(rows)

	repo := NewMySQLTicketRepository(db)
	tk, err := repo.Get(context.Background(), 999)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if tk != nil {
		t.Errorf("expected nil, got ticket with ID %d", tk.ID())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestMySQLTicketRepository_GetByCode_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	now := time.Now().Truncate(time.Second)
	rows := sqlmock.NewRows([]string{"id", "code", "event_id", "purchase_id", "status", "used_at", "created_at", "updated_at"}).
		AddRow(5, "xyz-789", 10, 100, "used", &now, now, now)

	mock.ExpectQuery("SELECT id, code, event_id, purchase_id, status, used_at, created_at, updated_at FROM tickets WHERE code = \\?").
		WithArgs("xyz-789").
		WillReturnRows(rows)

	repo := NewMySQLTicketRepository(db)
	tk, err := repo.GetByCode(context.Background(), "xyz-789")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if tk == nil {
		t.Fatal("expected ticket, got nil")
	}

	if tk.Status() != ticket.TicketStatusUsed {
		t.Errorf("expected status used, got %q", tk.Status())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestMySQLTicketRepository_Add_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	tk, _ := ticket.NewTicket(0, 10, 100)

	mock.ExpectExec("INSERT INTO tickets").
		WithArgs(tk.Code(), tk.EventID(), tk.PurchaseID(), string(tk.Status()), tk.CreatedAt(), tk.UpdatedAt()).
		WillReturnResult(sqlmock.NewResult(1000, 1))

	repo := NewMySQLTicketRepository(db)
	err = repo.Add(context.Background(), tk)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if tk.ID() != 1000 {
		t.Errorf("expected ID 1000 after Add, got %d", tk.ID())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestMySQLTicketRepository_Add_DBError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	tk, _ := ticket.NewTicket(0, 10, 100)

	mock.ExpectExec("INSERT INTO tickets").
		WithArgs(tk.Code(), tk.EventID(), tk.PurchaseID(), string(tk.Status()), tk.CreatedAt(), tk.UpdatedAt()).
		WillReturnError(errors.New("duplicate entry"))

	repo := NewMySQLTicketRepository(db)
	err = repo.Add(context.Background(), tk)

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestMySQLTicketRepository_Update_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	tk, _ := ticket.NewTicket(1, 10, 100)

	mock.ExpectExec("UPDATE tickets SET").
		WithArgs(string(tk.Status()), tk.UsedAt(), tk.UpdatedAt(), tk.ID()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	repo := NewMySQLTicketRepository(db)
	err = repo.Update(context.Background(), tk)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestMySQLTicketRepository_FindByPurchaseID_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	now := time.Now().Truncate(time.Second)
	rows := sqlmock.NewRows([]string{"id", "code", "event_id", "purchase_id", "status", "used_at", "created_at", "updated_at"}).
		AddRow(1, "code-1", 10, 100, "emitted", nil, now, now).
		AddRow(2, "code-2", 10, 100, "emitted", nil, now, now)

	mock.ExpectQuery("SELECT id, code, event_id, purchase_id, status, used_at, created_at, updated_at FROM tickets WHERE purchase_id = \\?").
		WithArgs(100).
		WillReturnRows(rows)

	repo := NewMySQLTicketRepository(db)
	tickets, err := repo.FindByPurchaseID(context.Background(), 100)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(tickets) != 2 {
		t.Errorf("expected 2 tickets, got %d", len(tickets))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestMySQLTicketRepository_FindByEventID_Empty(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id", "code", "event_id", "purchase_id", "status", "used_at", "created_at", "updated_at"})

	mock.ExpectQuery("SELECT id, code, event_id, purchase_id, status, used_at, created_at, updated_at FROM tickets WHERE event_id = \\?").
		WithArgs(999).
		WillReturnRows(rows)

	repo := NewMySQLTicketRepository(db)
	tickets, err := repo.FindByEventID(context.Background(), 999)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(tickets) != 0 {
		t.Errorf("expected 0 tickets, got %d", len(tickets))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
