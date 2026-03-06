package storage

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/iponzi/entradasQR/internal/ticket"
)

func TestMySQLPurchaseRepository_Get_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	now := time.Now().Truncate(time.Second)
	rows := sqlmock.NewRows([]string{"id", "buyer_email", "event_id", "quantity", "total_price", "created_at"}).
		AddRow(1, "buyer@test.com", 10, 3, 300.0, now)

	mock.ExpectQuery("SELECT id, buyer_email, event_id, quantity, total_price, created_at FROM purchases WHERE id = \\?").
		WithArgs(1).
		WillReturnRows(rows)

	repo := NewMySQLPurchaseRepository(db)
	purchase, err := repo.Get(context.Background(), 1)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if purchase == nil {
		t.Fatal("expected purchase, got nil")
	}

	if purchase.BuyerEmail() != "buyer@test.com" {
		t.Errorf("expected email 'buyer@test.com', got %q", purchase.BuyerEmail())
	}

	if purchase.Quantity() != 3 {
		t.Errorf("expected quantity 3, got %d", purchase.Quantity())
	}

	if purchase.TotalPrice() != 300.0 {
		t.Errorf("expected total price 300.0, got %f", purchase.TotalPrice())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestMySQLPurchaseRepository_Get_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id", "buyer_email", "event_id", "quantity", "total_price", "created_at"})

	mock.ExpectQuery("SELECT id, buyer_email, event_id, quantity, total_price, created_at FROM purchases WHERE id = \\?").
		WithArgs(999).
		WillReturnRows(rows)

	repo := NewMySQLPurchaseRepository(db)
	purchase, err := repo.Get(context.Background(), 999)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if purchase != nil {
		t.Errorf("expected nil, got purchase with ID %d", purchase.ID())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestMySQLPurchaseRepository_Get_DBError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery("SELECT id, buyer_email, event_id, quantity, total_price, created_at FROM purchases WHERE id = \\?").
		WithArgs(1).
		WillReturnError(errors.New("connection refused"))

	repo := NewMySQLPurchaseRepository(db)
	_, err = repo.Get(context.Background(), 1)

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestMySQLPurchaseRepository_Add_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	purchase, _ := ticket.NewPurchase(0, "buyer@test.com", 10, 3, 300.0)

	mock.ExpectExec("INSERT INTO purchases").
		WithArgs(purchase.BuyerEmail(), purchase.EventID(), purchase.Quantity(), purchase.TotalPrice(), purchase.CreatedAt()).
		WillReturnResult(sqlmock.NewResult(1000, 1))

	repo := NewMySQLPurchaseRepository(db)
	err = repo.Add(context.Background(), purchase)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if purchase.ID() != 1000 {
		t.Errorf("expected ID 1000 after Add, got %d", purchase.ID())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestMySQLPurchaseRepository_Add_DBError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	purchase, _ := ticket.NewPurchase(0, "buyer@test.com", 10, 3, 300.0)

	mock.ExpectExec("INSERT INTO purchases").
		WithArgs(purchase.BuyerEmail(), purchase.EventID(), purchase.Quantity(), purchase.TotalPrice(), purchase.CreatedAt()).
		WillReturnError(errors.New("table does not exist"))

	repo := NewMySQLPurchaseRepository(db)
	err = repo.Add(context.Background(), purchase)

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
