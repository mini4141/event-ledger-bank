// Package kafkax builds franz-go clients for Kafka or Amazon MSK Serverless (config only).
package kafkax

import (
	"context"
	"crypto/tls"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/sasl/aws"
)

const (
	TopicLedgerPosted      = "ledger.posted"
	TopicTransferCompleted = "transfer.completed"
	TopicTransferFailed    = "transfer.failed"
)

// Topics lists every topic in the system.
var Topics = []string{TopicLedgerPosted, TopicTransferCompleted, TopicTransferFailed}

// Opts returns base client options from KAFKA_BROKERS and KAFKA_SASL=none|aws-msk-iam.
func Opts() ([]kgo.Opt, error) {
	brokers := os.Getenv("KAFKA_BROKERS")
	if brokers == "" {
		brokers = "localhost:9092"
	}
	opts := []kgo.Opt{kgo.SeedBrokers(strings.Split(brokers, ",")...)}
	if os.Getenv("KAFKA_SASL") == "aws-msk-iam" {
		cfg, err := config.LoadDefaultConfig(context.Background())
		if err != nil {
			return nil, fmt.Errorf("aws config: %w", err)
		}
		opts = append(opts,
			kgo.DialTLSConfig(&tls.Config{MinVersion: tls.VersionTLS12}),
			kgo.SASL(aws.ManagedStreamingIAM(func(ctx context.Context) (aws.Auth, error) {
				c, err := cfg.Credentials.Retrieve(ctx)
				if err != nil {
					return aws.Auth{}, err
				}
				return aws.Auth{AccessKey: c.AccessKeyID, SecretKey: c.SecretAccessKey, SessionToken: c.SessionToken}, nil
			})),
		)
	}
	return opts, nil
}

// Producer creates a producing client.
func Producer() (*kgo.Client, error) {
	o, err := Opts()
	if err != nil {
		return nil, err
	}
	return kgo.NewClient(append(o, kgo.ProduceRequestTimeout(5*time.Second))...)
}

// Consumer creates a group consumer that commits offsets manually, after processing.
func Consumer(group string, topics ...string) (*kgo.Client, error) {
	o, err := Opts()
	if err != nil {
		return nil, err
	}
	return kgo.NewClient(append(o,
		kgo.ConsumerGroup(group),
		kgo.ConsumeTopics(topics...),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
		kgo.DisableAutoCommit(),
	)...)
}

// Publish sends one record synchronously.
func Publish(ctx context.Context, cl *kgo.Client, topic, key string, value []byte) error {
	return cl.ProduceSync(ctx, &kgo.Record{Topic: topic, Key: []byte(key), Value: value}).FirstErr()
}
