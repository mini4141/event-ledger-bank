//go:build integration

// Runs against the compose stack: gRPC through Envoy, Cassandra behind it, event read from Kafka.
package integration

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kgo"
	"google.golang.org/protobuf/proto"

	bankv1 "github.com/mini4141/event-ledger-bank/gen/bank/v1"
	"github.com/mini4141/event-ledger-bank/internal/grpcx"
)

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func TestTransferThroughEnvoyPublishesEvent(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	conn, err := grpcx.Dial(env("ENVOY_GRPC_ADDR", "localhost:18081"))
	require.NoError(t, err)
	acc := bankv1.NewAccountsServiceClient(conn)
	xfer := bankv1.NewTransfersServiceClient(conn)

	open := func(name string) string {
		c, err := acc.CreateCustomer(ctx, &bankv1.CreateCustomerRequest{Name: name})
		require.NoError(t, err)
		a, err := acc.OpenAccount(ctx, &bankv1.OpenAccountRequest{CustomerId: c.CustomerId, Currency: "GBP"})
		require.NoError(t, err)
		return a.AccountId
	}
	alice, bob := open("it-alice"), open("it-bob")
	_, err = acc.Deposit(ctx, &bankv1.DepositRequest{AccountId: alice, AmountMinor: 10000, IdempotencyKey: "it-" + alice})
	require.NoError(t, err)

	req := &bankv1.CreateTransferRequest{IdempotencyKey: "it-xfer-" + alice, FromAccountId: alice, ToAccountId: bob, AmountMinor: 2500}
	first, err := xfer.CreateTransfer(ctx, req)
	require.NoError(t, err)
	require.Equal(t, "COMPLETED", first.Status)

	replay, err := xfer.CreateTransfer(ctx, req)
	require.NoError(t, err)
	require.Equal(t, first.TransferId, replay.TransferId)
	require.True(t, replay.Replayed)

	a, err := acc.GetAccount(ctx, &bankv1.GetAccountRequest{AccountId: alice})
	require.NoError(t, err)
	b, err := acc.GetAccount(ctx, &bankv1.GetAccountRequest{AccountId: bob})
	require.NoError(t, err)
	require.Equal(t, int64(7500), a.BalanceMinor)
	require.Equal(t, int64(2500), b.BalanceMinor)

	cl, err := kgo.NewClient(kgo.SeedBrokers(env("KAFKA_BROKERS", "localhost:29092")),
		kgo.ConsumeTopics("transfer.completed"), kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()))
	require.NoError(t, err)
	defer cl.Close()
	for ctx.Err() == nil {
		fetches := cl.PollFetches(ctx)
		for _, rec := range fetches.Records() {
			var ev bankv1.TransferCompleted
			require.NoError(t, proto.Unmarshal(rec.Value, &ev))
			if ev.TransferId == first.TransferId {
				require.Equal(t, int64(2500), ev.AmountMinor)
				require.Equal(t, alice, string(rec.Key), "keyed by account id")
				return
			}
		}
	}
	t.Fatal("transfer.completed event not consumed")
}
