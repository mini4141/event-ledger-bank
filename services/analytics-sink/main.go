// analytics-sink: consumes every topic and writes hourly-partitioned NDJSON to a folder or S3.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/twmb/franz-go/pkg/kgo"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	bankv1 "github.com/mini4141/event-ledger-bank/gen/bank/v1"
	"github.com/mini4141/event-ledger-bank/internal/kafkax"
	"github.com/mini4141/event-ledger-bank/internal/obs"
)

// toRow decodes a protobuf record into a flat JSON row (snake_case, numeric amounts).
func toRow(rec *kgo.Record) ([]byte, error) {
	var m proto.Message
	switch rec.Topic {
	case kafkax.TopicLedgerPosted:
		m = &bankv1.LedgerPosted{}
	case kafkax.TopicTransferCompleted:
		m = &bankv1.TransferCompleted{}
	case kafkax.TopicTransferFailed:
		m = &bankv1.TransferFailed{}
	default:
		return nil, fmt.Errorf("unknown topic %s", rec.Topic)
	}
	if err := proto.Unmarshal(rec.Value, m); err != nil {
		return nil, err
	}
	js, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(m)
	if err != nil {
		return nil, err
	}
	row := map[string]any{}
	if err := json.Unmarshal(js, &row); err != nil {
		return nil, err
	}
	if s, ok := row["amount_minor"].(string); ok { // int64 is a JSON string in protojson
		if n, err := strconv.ParseInt(s, 10, 64); err == nil {
			row["amount_minor"] = n
		}
	}
	row["topic"] = rec.Topic
	row["key"] = string(rec.Key)
	return json.Marshal(row)
}

type writer struct {
	dir    string
	bucket string
	s3c    *s3.Client
}

func (w *writer) write(ctx context.Context, body []byte) error {
	now := time.Now().UTC()
	rel := fmt.Sprintf("dt=%s/hour=%s/events-%d.ndjson", now.Format("2006-01-02"), now.Format("15"), now.UnixNano())
	if w.bucket != "" {
		_, err := w.s3c.PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(w.bucket), Key: aws.String("events/" + rel), Body: bytes.NewReader(body)})
		return err
	}
	p := filepath.Join(w.dir, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, body, 0o644)
}

func main() {
	ctx := context.Background()
	shutdown := obs.Init(ctx, "analytics-sink")
	defer shutdown(ctx)
	obs.ServeHTTP(":8080", nil)

	w := &writer{dir: os.Getenv("SINK_DIR"), bucket: os.Getenv("SINK_S3_BUCKET")}
	if w.bucket != "" {
		cfg, err := config.LoadDefaultConfig(ctx)
		if err != nil {
			slog.Error("aws config", "err", err)
			os.Exit(1)
		}
		w.s3c = s3.NewFromConfig(cfg)
	} else if w.dir == "" {
		w.dir = "/data/sink"
	}

	cl, err := kafkax.Consumer("analytics-sink", kafkax.Topics...)
	if err != nil {
		slog.Error("kafka", "err", err)
		os.Exit(1)
	}
	for {
		var buf bytes.Buffer
		n := 0
		deadline := time.Now().Add(5 * time.Second) // batch up to 5s so files are not tiny
		for time.Now().Before(deadline) && n < 2000 {
			pctx, cancel := context.WithDeadline(ctx, deadline)
			fetches := cl.PollRecords(pctx, 500)
			cancel()
			if fetches.IsClientClosed() {
				return
			}
			for _, rec := range fetches.Records() {
				row, err := toRow(rec)
				if err != nil {
					slog.Error("decode", "topic", rec.Topic, "err", err)
					continue
				}
				buf.Write(row)
				buf.WriteByte('\n')
				n++
			}
		}
		if n == 0 {
			continue
		}
		if err := w.write(ctx, buf.Bytes()); err != nil {
			slog.Error("write", "err", err) // offsets stay uncommitted: records are redelivered
			continue
		}
		if err := cl.CommitUncommittedOffsets(ctx); err != nil {
			slog.Error("commit", "err", err)
		}
		slog.Info("flushed", "records", n)
	}
}
