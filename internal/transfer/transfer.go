// Package transfer orchestrates idempotent transfers over injected dependencies.
package transfer

import (
	"context"

	"github.com/google/uuid"
)

// Record is the stored idempotency row.
type Record struct {
	TransferID string
	Status     string
}

// Store claims idempotency keys (Cassandra LWT: INSERT ... IF NOT EXISTS).
type Store interface {
	// Claim returns applied=true when the key was new; otherwise the existing record.
	Claim(ctx context.Context, key, transferID string) (applied bool, existing Record, err error)
	SetStatus(ctx context.Context, key, status string) error
}

// Ledger posts a balanced double entry.
type Ledger interface {
	Post(ctx context.Context, postingID, from, to string, amount int64) error
}

// Events publishes transfer events.
type Events interface {
	Completed(ctx context.Context, transferID, from, to string, amount int64)
	Failed(ctx context.Context, transferID, from, to string, amount int64, reason string)
}

type Service struct {
	Store  Store
	Ledger Ledger
	Events Events
}

// Result is the outcome of Create.
type Result struct {
	TransferID string
	Status     string
	Replayed   bool
}

// Create runs a transfer once per idempotency key; a replay returns the original result.
func (s *Service) Create(ctx context.Context, key, from, to string, amount int64) (Result, error) {
	id := uuid.NewString()
	applied, existing, err := s.Store.Claim(ctx, key, id)
	if err != nil {
		return Result{}, err
	}
	if !applied {
		return Result{TransferID: existing.TransferID, Status: existing.Status, Replayed: true}, nil
	}
	if err := s.Ledger.Post(ctx, id, from, to, amount); err != nil {
		_ = s.Store.SetStatus(ctx, key, "FAILED")
		s.Events.Failed(ctx, id, from, to, amount, err.Error())
		return Result{TransferID: id, Status: "FAILED"}, err
	}
	if err := s.Store.SetStatus(ctx, key, "COMPLETED"); err != nil {
		return Result{}, err
	}
	s.Events.Completed(ctx, id, from, to, amount)
	return Result{TransferID: id, Status: "COMPLETED"}, nil
}
