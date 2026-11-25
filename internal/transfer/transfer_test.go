package transfer

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type memStore struct{ m map[string]Record }

func (s *memStore) Claim(_ context.Context, key, id string) (bool, Record, error) {
	if r, ok := s.m[key]; ok {
		return false, r, nil
	}
	s.m[key] = Record{TransferID: id, Status: "PENDING"}
	return true, Record{}, nil
}

func (s *memStore) SetStatus(_ context.Context, key, st string) error {
	r := s.m[key]
	r.Status = st
	s.m[key] = r
	return nil
}

type countLedger struct{ n int }

func (l *countLedger) Post(context.Context, string, string, string, int64) error { l.n++; return nil }

type noEvents struct{}

func (noEvents) Completed(context.Context, string, string, string, int64)     {}
func (noEvents) Failed(context.Context, string, string, string, int64, string) {}

func TestIdempotentReplayReturnsOriginal(t *testing.T) {
	l := &countLedger{}
	s := &Service{Store: &memStore{m: map[string]Record{}}, Ledger: l, Events: noEvents{}}
	first, err := s.Create(context.Background(), "k1", "a", "b", 2500)
	require.NoError(t, err)
	again, err := s.Create(context.Background(), "k1", "a", "b", 2500)
	require.NoError(t, err)
	require.Equal(t, first.TransferID, again.TransferID)
	require.True(t, again.Replayed)
	require.Equal(t, 1, l.n, "ledger must be posted once")
}
