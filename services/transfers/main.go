// transfers: idempotent transfer orchestration; calls ledger through Envoy.
package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/gocql/gocql"
	"github.com/twmb/franz-go/pkg/kgo"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	bankv1 "github.com/mini4141/event-ledger-bank/gen/bank/v1"
	"github.com/mini4141/event-ledger-bank/internal/cass"
	"github.com/mini4141/event-ledger-bank/internal/grpcx"
	"github.com/mini4141/event-ledger-bank/internal/kafkax"
	"github.com/mini4141/event-ledger-bank/internal/obs"
	"github.com/mini4141/event-ledger-bank/internal/transfer"
)

type store struct{ db *gocql.Session }

func (s store) Claim(ctx context.Context, key, id string) (bool, transfer.Record, error) {
	prev := map[string]any{}
	applied, err := s.db.Query(`INSERT INTO transfers_by_idempotency_key (idempotency_key, transfer_id, status, created_at) VALUES (?, ?, 'PENDING', ?) IF NOT EXISTS`,
		key, id, time.Now().UTC()).WithContext(ctx).MapScanCAS(prev)
	if err != nil || applied {
		return applied, transfer.Record{}, err
	}
	tid, _ := prev["transfer_id"].(gocql.UUID)
	st, _ := prev["status"].(string)
	return false, transfer.Record{TransferID: tid.String(), Status: st}, nil
}

func (s store) SetStatus(ctx context.Context, key, st string) error {
	return s.db.Query(`UPDATE transfers_by_idempotency_key SET status = ? WHERE idempotency_key = ?`, st, key).WithContext(ctx).Exec()
}

type ledgerClient struct{ c bankv1.LedgerServiceClient }

func (l ledgerClient) Post(ctx context.Context, postingID, from, to string, amount int64) error {
	_, err := l.c.Post(ctx, &bankv1.PostRequest{PostingId: postingID, FromAccountId: from, ToAccountId: to, AmountMinor: amount})
	return err
}

type events struct{ kc *kgo.Client }

func (e events) Completed(ctx context.Context, id, from, to string, amount int64) {
	b, _ := proto.Marshal(&bankv1.TransferCompleted{TransferId: id, FromAccountId: from, ToAccountId: to, AmountMinor: amount, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano)})
	if err := kafkax.Publish(ctx, e.kc, kafkax.TopicTransferCompleted, from, b); err != nil {
		slog.ErrorContext(ctx, "publish transfer.completed", "err", err)
	}
}

func (e events) Failed(ctx context.Context, id, from, to string, amount int64, reason string) {
	b, _ := proto.Marshal(&bankv1.TransferFailed{TransferId: id, FromAccountId: from, ToAccountId: to, AmountMinor: amount, Reason: reason, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano)})
	if err := kafkax.Publish(ctx, e.kc, kafkax.TopicTransferFailed, from, b); err != nil {
		slog.ErrorContext(ctx, "publish transfer.failed", "err", err)
	}
}

type server struct {
	bankv1.UnimplementedTransfersServiceServer
	svc *transfer.Service
}

func (s *server) CreateTransfer(ctx context.Context, r *bankv1.CreateTransferRequest) (*bankv1.Transfer, error) {
	if r.IdempotencyKey == "" || r.AmountMinor <= 0 || r.FromAccountId == "" || r.ToAccountId == "" {
		return nil, status.Error(codes.InvalidArgument, "idempotency_key, accounts and positive amount required")
	}
	res, err := s.svc.Create(ctx, r.IdempotencyKey, r.FromAccountId, r.ToAccountId, r.AmountMinor)
	if err != nil {
		return nil, err
	}
	return &bankv1.Transfer{TransferId: res.TransferID, Status: res.Status, Replayed: res.Replayed}, nil
}

func main() {
	ctx := context.Background()
	shutdown := obs.Init(ctx, "transfers")
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
	conn, err := grpcx.Dial(os.Getenv("ENVOY_GRPC_ADDR"))
	if err != nil {
		slog.Error("dial envoy", "err", err)
		os.Exit(1)
	}
	svc := &transfer.Service{Store: store{db}, Ledger: ledgerClient{bankv1.NewLedgerServiceClient(conn)}, Events: events{kc}}
	srv := grpcx.NewServer()
	bankv1.RegisterTransfersServiceServer(srv, &server{svc: svc})
	if err := grpcx.Serve(srv, ":9000"); err != nil {
		slog.Error("serve", "err", err)
		os.Exit(1)
	}
}
