package storage

import (
	"context"
	"testing"

	"github.com/iponzi/entradasQR/internal/validator"
	"github.com/redis/go-redis/v9"
)

func setupTestRedis(t *testing.T) *redis.Client {
	t.Helper()

	client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})

	if err := client.Ping(context.Background()).Err(); err != nil {
		t.Skipf("redis not available, skipping: %v", err)
	}

	t.Cleanup(func() {
		client.FlushDB(context.Background())
		client.Close()
	})

	return client
}

func TestRedisValidTicketRepository_AddAndGetByCode(t *testing.T) {
	client := setupTestRedis(t)
	repo := NewRedisValidTicketRepository(client)
	ctx := context.Background()

	vt, err := validator.NewValidTicket(0, "abc-123", 10)
	if err != nil {
		t.Fatalf("failed to create valid ticket: %v", err)
	}

	if err := repo.Add(ctx, vt); err != nil {
		t.Fatalf("failed to add ticket: %v", err)
	}

	got, err := repo.GetByCode(ctx, "abc-123")
	if err != nil {
		t.Fatalf("failed to get ticket: %v", err)
	}

	if got == nil {
		t.Fatal("expected ticket, got nil")
	}

	if got.Code() != "abc-123" {
		t.Errorf("expected code 'abc-123', got %q", got.Code())
	}

	if got.EventID() != 10 {
		t.Errorf("expected eventID 10, got %d", got.EventID())
	}

	if got.Status() != validator.ValidTicketStatusActive {
		t.Errorf("expected status 'active', got %q", got.Status())
	}
}

func TestRedisValidTicketRepository_GetByCode_NotFound(t *testing.T) {
	client := setupTestRedis(t)
	repo := NewRedisValidTicketRepository(client)
	ctx := context.Background()

	got, err := repo.GetByCode(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got != nil {
		t.Error("expected nil for nonexistent ticket")
	}
}

func TestRedisValidTicketRepository_Update(t *testing.T) {
	client := setupTestRedis(t)
	repo := NewRedisValidTicketRepository(client)
	ctx := context.Background()

	vt, _ := validator.NewValidTicket(0, "abc-456", 20)

	if err := repo.Add(ctx, vt); err != nil {
		t.Fatalf("failed to add ticket: %v", err)
	}

	if err := vt.MarkAsUsed(); err != nil {
		t.Fatalf("failed to mark as used: %v", err)
	}

	if err := repo.Update(ctx, vt); err != nil {
		t.Fatalf("failed to update ticket: %v", err)
	}

	got, err := repo.GetByCode(ctx, "abc-456")
	if err != nil {
		t.Fatalf("failed to get ticket: %v", err)
	}

	if got.Status() != validator.ValidTicketStatusUsed {
		t.Errorf("expected status 'used', got %q", got.Status())
	}

	if got.UsedAt() == nil {
		t.Error("expected usedAt to be set")
	}
}

func TestRedisValidTicketRepository_Get_ReturnsNil(t *testing.T) {
	client := setupTestRedis(t)
	repo := NewRedisValidTicketRepository(client)
	ctx := context.Background()

	got, err := repo.Get(ctx, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got != nil {
		t.Error("expected nil from Get by ID (not supported in Redis)")
	}
}
