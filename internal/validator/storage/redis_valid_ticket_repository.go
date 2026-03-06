package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/iponzi/entradasQR/internal/validator"
)

const redisKeyPrefix = "ticket:"

// redisTicketRecord is the JSON value stored in Redis for each valid ticket.
type redisTicketRecord struct {
	EventID   int                         `json:"event_id"`
	Status    validator.ValidTicketStatus `json:"status"`
	UsedAt    *time.Time                  `json:"used_at,omitempty"`
	SyncedAt  time.Time                   `json:"synced_at"`
	UpdatedAt time.Time                   `json:"updated_at"`
}

// RedisValidTicketRepository implements ValidTicketRepository using Redis.
// Key format: "ticket:{code}" with a JSON value.
type RedisValidTicketRepository struct {
	client *redis.Client
}

// NewRedisValidTicketRepository creates a new Redis-backed repository.
func NewRedisValidTicketRepository(client *redis.Client) *RedisValidTicketRepository {
	return &RedisValidTicketRepository{client: client}
}

// Get retrieves a valid ticket by ID. Not supported in Redis (ID is MySQL-specific).
// Always returns nil since Redis keys are by code.
func (r *RedisValidTicketRepository) Get(_ context.Context, _ int) (*validator.ValidTicket, error) {
	return nil, nil
}

// GetByCode retrieves a valid ticket by its code from Redis.
func (r *RedisValidTicketRepository) GetByCode(ctx context.Context, code string) (*validator.ValidTicket, error) {
	key := redisKeyPrefix + code

	data, err := r.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("redis get %s: %w", key, err)
	}

	var record redisTicketRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, fmt.Errorf("unmarshaling ticket record: %w", err)
	}

	return validator.NewValidTicketFromRepository(
		0, code, record.EventID, record.Status,
		record.UsedAt, record.SyncedAt, record.UpdatedAt,
	), nil
}

// Add persists a new valid ticket to Redis.
func (r *RedisValidTicketRepository) Add(ctx context.Context, ticket *validator.ValidTicket) error {
	key := redisKeyPrefix + ticket.Code()

	record := redisTicketRecord{
		EventID:   ticket.EventID(),
		Status:    ticket.Status(),
		UsedAt:    ticket.UsedAt(),
		SyncedAt:  ticket.SyncedAt(),
		UpdatedAt: ticket.UpdatedAt(),
	}

	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshaling ticket record: %w", err)
	}

	if err := r.client.Set(ctx, key, data, 0).Err(); err != nil {
		return fmt.Errorf("redis set %s: %w", key, err)
	}

	return nil
}

// Update overwrites an existing valid ticket in Redis.
func (r *RedisValidTicketRepository) Update(ctx context.Context, ticket *validator.ValidTicket) error {
	return r.Add(ctx, ticket)
}
