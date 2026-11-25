// topic-init: creates the Kafka topics (local Kafka, kind and MSK Serverless) and exits.
package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"strconv"
	"time"

	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kerr"
	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/mini4141/event-ledger-bank/internal/kafkax"
	"github.com/mini4141/event-ledger-bank/internal/obs"
)

func main() {
	ctx := context.Background()
	obs.Init(ctx, "topic-init")
	opts, err := kafkax.Opts()
	if err != nil {
		slog.Error("opts", "err", err)
		os.Exit(1)
	}
	cl, err := kgo.NewClient(opts...)
	if err != nil {
		slog.Error("client", "err", err)
		os.Exit(1)
	}
	adm := kadm.NewClient(cl)
	parts, _ := strconv.Atoi(os.Getenv("TOPIC_PARTITIONS"))
	if parts == 0 {
		parts = 3
	}
	var lastErr error
	for i := 0; i < 30; i++ {
		var res kadm.CreateTopicResponses
		res, lastErr = adm.CreateTopics(ctx, int32(parts), -1, nil, kafkax.Topics...)
		if lastErr == nil {
			for _, r := range res.Sorted() {
				if r.Err != nil && !errors.Is(r.Err, kerr.TopicAlreadyExists) {
					lastErr = r.Err
					break
				}
				slog.Info("topic ready", "topic", r.Topic, "partitions", parts, "err", r.Err)
			}
		}
		if lastErr == nil {
			return
		}
		slog.Warn("retrying", "err", lastErr)
		time.Sleep(3 * time.Second)
	}
	slog.Error("topics", "err", lastErr)
	os.Exit(1)
}
