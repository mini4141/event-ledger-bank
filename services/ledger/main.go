// ledger: double-entry postings in Cassandra (LWT on version) + ledger.posted events.
package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"time"

	"github.com/gocql/gocql"
	"github.com/twmb/franz-go/pkg/kgo"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	bankv1 "github.com/mini4141/event-ledger-bank/gen/bank/v1"
	"github.com/mini4141/event-ledger-bank/internal/bank"
	"github.com/mini4141/event-ledger-bank/internal/cass"
	"github.com/mini4141/event-ledger-bank/internal/grpcx"
	"github.com/mini4141/event-ledger-bank/internal/kafkax"
	"github.com/mini4141/event-ledger-bank/internal/obs"
)

const maxCASAttempts = 8

type server struct {
	bankv1.UnimplementedLedgerServiceServer
	db *gocql.Session
	kc *kgo.Client
}

// apply moves delta onto an account with a conditional update on version (LWT), retrying on conflict.
func (s *server) apply(ctx context.Context, accountID string, delta int64) (int64, error) {
	for i := 0; i < maxCASAttempts; i++ {
		var bal int64
		var ver int
		if err := s.db.Query(`SELECT balance_minor, version FROM accounts WHERE account_id = ?`, accountID).WithContext(ctx).Scan(&bal, &ver); err != nil {
			if errors.Is(err, gocql.ErrNotFound) {
				return 0, status.Error(codes.NotFound, "account not found: "+accountID)
			}
			return 0, status.Error(codes.Unavailable, err.Error())
		}
		nb, err := bank.NewBalance(accountID, bal, delta)
		if err != nil {
			return 0, status.Error(codes.FailedPrecondition, err.Error())
		}
		applied, err := s.db.Query(`UPDATE accounts SET balance_minor = ?, version = ? WHERE account_id = ? IF version = ?`,
			nb, ver+1, accountID, ver).WithContext(ctx).MapScanCAS(map[string]any{})
		if err != nil {
			return 0, status.Error(codes.Unavailable, err.Error())
		}
		if applied {
			return nb, nil
		}
		time.Sleep(time.Duration(5+i*5) * time.Millisecond) // version conflict: re-read and retry
	}
	return 0, status.Error(codes.Aborted, "too many version conflicts")
}

func (s *server) setMarker(ctx context.Context, postingID, st string) error {
	return s.db.Query(`UPDATE postings_by_id SET status = ? WHERE posting_id = ?`, st, postingID).WithContext(ctx).Exec()
}

func (s *server) Post(ctx context.Context, r *bankv1.PostRequest) (*bankv1.PostResponse, error) {
	if r.AmountMinor <= 0 || r.PostingId == "" || r.FromAccountId == r.ToAccountId {
		return nil, status.Error(codes.InvalidArgument, "invalid posting")
	}
	// Idempotency marker so an Envoy retry cannot double-post.
	prev := map[string]any{}
	applied, err := s.db.Query(`INSERT INTO postings_by_id (posting_id, status, created_at) VALUES (?, 'POSTING', ?) IF NOT EXISTS`,
		r.PostingId, time.Now().UTC()).WithContext(ctx).MapScanCAS(prev)
	if err != nil {
		return nil, status.Error(codes.Unavailable, err.Error())
	}
	if !applied {
		switch prev["status"] {
		case "POSTED":
			slog.InfoContext(ctx, "posting replay", "posting_id", r.PostingId)
			return &bankv1.PostResponse{PostingId: r.PostingId}, nil
		case "REJECTED":
			return nil, status.Error(codes.FailedPrecondition, bank.ErrInsufficientFunds.Error())
		default:
			return nil, status.Error(codes.Aborted, "posting in progress")
		}
	}

	e := bank.Entries(r.FromAccountId, r.ToAccountId, r.AmountMinor)
	fromBal, err := s.apply(ctx, e[0].AccountID, e[0].Delta)
	if err != nil {
		_ = s.setMarker(ctx, r.PostingId, "REJECTED")
		return nil, err
	}
	toBal, err := s.apply(ctx, e[1].AccountID, e[1].Delta)
	if err != nil { // compensate the debit so the books stay balanced
		if _, cerr := s.apply(ctx, e[0].AccountID, -e[0].Delta); cerr != nil {
			slog.ErrorContext(ctx, "compensation failed", "posting_id", r.PostingId, "err", cerr)
		}
		_ = s.setMarker(ctx, r.PostingId, "REJECTED")
		return nil, err
	}
	now := time.Now().UTC()
	for _, en := range e {
		if err := s.db.Query(`INSERT INTO ledger_entries_by_account (account_id, posted_at, posting_id, direction, amount_minor, counterparty) VALUES (?, ?, ?, ?, ?, ?)`,
			en.AccountID, gocql.UUIDFromTime(now), r.PostingId, en.Direction, en.AmountMinor, en.Counterparty).WithContext(ctx).Exec(); err != nil {
			return nil, status.Error(codes.Unavailable, err.Error())
		}
	}
	if err := s.setMarker(ctx, r.PostingId, "POSTED"); err != nil {
		return nil, status.Error(codes.Unavailable, err.Error())
	}
	for _, en := range e {
		ev := &bankv1.LedgerPosted{PostingId: r.PostingId, AccountId: en.AccountID, Counterparty: en.Counterparty,
			Direction: en.Direction, AmountMinor: en.AmountMinor, OccurredAt: now.Format(time.RFC3339Nano)}
		b, _ := proto.Marshal(ev)
		if err := kafkax.Publish(ctx, s.kc, kafkax.TopicLedgerPosted, en.AccountID, b); err != nil {
			slog.ErrorContext(ctx, "publish ledger.posted", "err", err) // best effort; outbox is out of scope
		}
	}
	return &bankv1.PostResponse{PostingId: r.PostingId, FromBalanceMinor: fromBal, ToBalanceMinor: toBal}, nil
}

func (s *server) ListEntries(ctx context.Context, r *bankv1.ListEntriesRequest) (*bankv1.ListEntriesResponse, error) {
	limit := int(r.Limit)
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	it := s.db.Query(`SELECT posted_at, posting_id, direction, amount_minor, counterparty FROM ledger_entries_by_account WHERE account_id = ? LIMIT ?`,
		r.AccountId, limit).WithContext(ctx).Iter()
	out := &bankv1.ListEntriesResponse{}
	var at gocql.UUID
	var pid, dir, cp string
	var amt int64
	for it.Scan(&at, &pid, &dir, &amt, &cp) {
		out.Entries = append(out.Entries, &bankv1.Entry{AccountId: r.AccountId, PostedAt: at.Time().UTC().Format(time.RFC3339Nano),
			PostingId: pid, Direction: dir, AmountMinor: amt, Counterparty: cp})
	}
	if err := it.Close(); err != nil {
		return nil, status.Error(codes.Unavailable, err.Error())
	}
	return out, nil
}

func main() {
	ctx := context.Background()
	shutdown := obs.Init(ctx, "ledger")
	defer shutdown(ctx)
	obs.ServeHTTP(":8080", nil)

	db, err := cass.Connect()
	if err != nil {
		slog.Error("cassandra", "err", err)
		os.Exit(1)
	}
	kc, err := kafkax.Producer()
	if err != nil {
		slog.Error("kafka", "err", err)
		os.Exit(1)
	}
	srv := grpcx.NewServer()
	bankv1.RegisterLedgerServiceServer(srv, &server{db: db, kc: kc})
	if err := grpcx.Serve(srv, ":9000"); err != nil {
		slog.Error("serve", "err", err)
		os.Exit(1)
	}
}
