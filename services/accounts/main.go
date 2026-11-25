// accounts: customers and accounts (Cassandra), deposits via the ledger.
package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/gocql/gocql"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	bankv1 "github.com/mini4141/event-ledger-bank/gen/bank/v1"
	"github.com/mini4141/event-ledger-bank/internal/bank"
	"github.com/mini4141/event-ledger-bank/internal/cass"
	"github.com/mini4141/event-ledger-bank/internal/grpcx"
	"github.com/mini4141/event-ledger-bank/internal/obs"
)

type server struct {
	bankv1.UnimplementedAccountsServiceServer
	db     *gocql.Session
	ledger bankv1.LedgerServiceClient
}

func (s *server) CreateCustomer(ctx context.Context, r *bankv1.CreateCustomerRequest) (*bankv1.Customer, error) {
	if r.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name required")
	}
	id, now := uuid.NewString(), time.Now().UTC()
	if err := s.db.Query(`INSERT INTO customers (customer_id, name, created_at) VALUES (?, ?, ?)`, id, r.Name, now).WithContext(ctx).Exec(); err != nil {
		return nil, status.Error(codes.Unavailable, err.Error())
	}
	return &bankv1.Customer{CustomerId: id, Name: r.Name, CreatedAt: now.Format(time.RFC3339)}, nil
}

func (s *server) ListCustomers(ctx context.Context, _ *bankv1.ListCustomersRequest) (*bankv1.ListCustomersResponse, error) {
	// Admin-tool full scan; acceptable at demo scale (see docs/data-model.md).
	it := s.db.Query(`SELECT customer_id, name, created_at FROM customers LIMIT 200`).WithContext(ctx).Iter()
	out := &bankv1.ListCustomersResponse{}
	var id, name string
	var ts time.Time
	for it.Scan(&id, &name, &ts) {
		out.Customers = append(out.Customers, &bankv1.Customer{CustomerId: id, Name: name, CreatedAt: ts.UTC().Format(time.RFC3339)})
	}
	if err := it.Close(); err != nil {
		return nil, status.Error(codes.Unavailable, err.Error())
	}
	return out, nil
}

func (s *server) OpenAccount(ctx context.Context, r *bankv1.OpenAccountRequest) (*bankv1.Account, error) {
	cur := r.Currency
	if cur == "" {
		cur = "GBP"
	}
	if _, err := uuid.Parse(r.CustomerId); err != nil {
		return nil, status.Error(codes.InvalidArgument, "bad customer_id")
	}
	id, now := uuid.NewString(), time.Now().UTC()
	if err := s.db.Query(`INSERT INTO accounts (account_id, customer_id, currency, balance_minor, version, created_at) VALUES (?, ?, ?, 0, 0, ?)`,
		id, r.CustomerId, cur, now).WithContext(ctx).Exec(); err != nil {
		return nil, status.Error(codes.Unavailable, err.Error())
	}
	if err := s.db.Query(`INSERT INTO accounts_by_customer (customer_id, account_id) VALUES (?, ?)`, r.CustomerId, id).WithContext(ctx).Exec(); err != nil {
		return nil, status.Error(codes.Unavailable, err.Error())
	}
	return &bankv1.Account{AccountId: id, CustomerId: r.CustomerId, Currency: cur, CreatedAt: now.Format(time.RFC3339)}, nil
}

func (s *server) GetAccount(ctx context.Context, r *bankv1.GetAccountRequest) (*bankv1.Account, error) {
	var cid, cur string
	var bal int64
	var ver int
	var ts time.Time
	err := s.db.Query(`SELECT customer_id, currency, balance_minor, version, created_at FROM accounts WHERE account_id = ?`, r.AccountId).
		WithContext(ctx).Scan(&cid, &cur, &bal, &ver, &ts)
	if err == gocql.ErrNotFound {
		return nil, status.Error(codes.NotFound, "account not found")
	}
	if err != nil {
		return nil, status.Error(codes.Unavailable, err.Error())
	}
	return &bankv1.Account{AccountId: r.AccountId, CustomerId: cid, Currency: cur, BalanceMinor: bal, Version: int32(ver), CreatedAt: ts.UTC().Format(time.RFC3339)}, nil
}

func (s *server) ListAccounts(ctx context.Context, r *bankv1.ListAccountsRequest) (*bankv1.ListAccountsResponse, error) {
	it := s.db.Query(`SELECT account_id FROM accounts_by_customer WHERE customer_id = ?`, r.CustomerId).WithContext(ctx).Iter()
	var ids []string
	var id string
	for it.Scan(&id) {
		ids = append(ids, id)
	}
	if err := it.Close(); err != nil {
		return nil, status.Error(codes.Unavailable, err.Error())
	}
	out := &bankv1.ListAccountsResponse{}
	for _, id := range ids {
		a, err := s.GetAccount(ctx, &bankv1.GetAccountRequest{AccountId: id})
		if err != nil {
			return nil, err
		}
		out.Accounts = append(out.Accounts, a)
	}
	return out, nil
}

// Deposit credits test funds from the bank_funding system account via the ledger.
func (s *server) Deposit(ctx context.Context, r *bankv1.DepositRequest) (*bankv1.Account, error) {
	if r.AmountMinor <= 0 {
		return nil, status.Error(codes.InvalidArgument, "amount must be positive")
	}
	postingID := uuid.NewString()
	if r.IdempotencyKey != "" {
		postingID = uuid.NewSHA1(uuid.NameSpaceURL, []byte("deposit:"+r.IdempotencyKey)).String()
	}
	if _, err := s.ledger.Post(ctx, &bankv1.PostRequest{
		PostingId: postingID, FromAccountId: bank.FundingAccountID, ToAccountId: r.AccountId, AmountMinor: r.AmountMinor,
	}); err != nil {
		return nil, err
	}
	return s.GetAccount(ctx, &bankv1.GetAccountRequest{AccountId: r.AccountId})
}

func (s *server) ensureFundingAccount() error {
	return s.db.Query(`INSERT INTO accounts (account_id, customer_id, currency, balance_minor, version, created_at) VALUES (?, ?, 'GBP', 0, 0, ?) IF NOT EXISTS`,
		bank.FundingAccountID, bank.FundingAccountID, time.Now().UTC()).Exec()
}

func main() {
	ctx := context.Background()
	shutdown := obs.Init(ctx, "accounts")
	defer shutdown(ctx)
	obs.ServeHTTP(":8080", nil)

	db, err := cass.Connect()
	if err != nil {
		slog.Error("cassandra", "err", err)
		os.Exit(1)
	}
	conn, err := grpcx.Dial(os.Getenv("ENVOY_GRPC_ADDR"))
	if err != nil {
		slog.Error("dial envoy", "err", err)
		os.Exit(1)
	}
	s := &server{db: db, ledger: bankv1.NewLedgerServiceClient(conn)}
	if err := s.ensureFundingAccount(); err != nil {
		slog.Error("funding account", "err", err)
		os.Exit(1)
	}
	srv := grpcx.NewServer()
	bankv1.RegisterAccountsServiceServer(srv, s)
	if err := grpcx.Serve(srv, ":9000"); err != nil {
		slog.Error("serve", "err", err)
		os.Exit(1)
	}
}
