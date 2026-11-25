// notifications: consumes transfer events, keeps the last 100 and streams them over SSE.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"

	bankv1 "github.com/mini4141/event-ledger-bank/gen/bank/v1"
	"github.com/mini4141/event-ledger-bank/internal/kafkax"
	"github.com/mini4141/event-ledger-bank/internal/obs"
)

const keep = 100

type event struct {
	Type          string `json:"type"`
	TransferID    string `json:"transfer_id"`
	FromAccountID string `json:"from_account_id"`
	ToAccountID   string `json:"to_account_id"`
	AmountMinor   int64  `json:"amount_minor"`
	Reason        string `json:"reason,omitempty"`
	OccurredAt    string `json:"occurred_at"`
}

type hub struct {
	mu     sync.Mutex
	recent []event
	subs   map[chan event]struct{}
}

func (h *hub) add(e event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.recent = append(h.recent, e)
	if len(h.recent) > keep {
		h.recent = h.recent[len(h.recent)-keep:]
	}
	for c := range h.subs {
		select {
		case c <- e:
		default: // slow subscriber: drop
		}
	}
}

func (h *hub) subscribe() (backlog []event, c chan event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	c = make(chan event, 64)
	h.subs[c] = struct{}{}
	return append([]event(nil), h.recent...), c
}

func (h *hub) unsubscribe(c chan event) {
	h.mu.Lock()
	delete(h.subs, c)
	h.mu.Unlock()
}

func (h *hub) list(w http.ResponseWriter, _ *http.Request) {
	h.mu.Lock()
	out := append([]event{}, h.recent...)
	h.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

func (h *hub) stream(w http.ResponseWriter, r *http.Request) {
	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	backlog, c := h.subscribe()
	defer h.unsubscribe(c)
	send := func(e event) {
		b, _ := json.Marshal(e)
		fmt.Fprintf(w, "data: %s\n\n", b)
	}
	fmt.Fprint(w, ": connected\n\n")
	for _, e := range backlog {
		send(e)
	}
	fl.Flush()
	tick := time.NewTicker(15 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case e := <-c:
			send(e)
		case <-tick.C:
			fmt.Fprint(w, ": ping\n\n")
		}
		fl.Flush()
	}
}

func main() {
	ctx := context.Background()
	shutdown := obs.Init(ctx, "notifications")
	defer shutdown(ctx)
	h := &hub{subs: map[chan event]struct{}{}}
	obs.ServeHTTP(":8080", func(mux *http.ServeMux) {
		mux.HandleFunc("/v1/feed/stream", h.stream)
		mux.HandleFunc("/v1/feed", h.list)
	})

	cl, err := kafkax.Consumer("notifications", kafkax.TopicTransferCompleted, kafkax.TopicTransferFailed)
	if err != nil {
		slog.Error("kafka", "err", err)
		os.Exit(1)
	}
	for {
		fetches := cl.PollFetches(ctx)
		if fetches.IsClientClosed() {
			return
		}
		fetches.EachError(func(t string, p int32, err error) { slog.Error("fetch", "topic", t, "partition", p, "err", err) })
		for _, rec := range fetches.Records() {
			switch rec.Topic {
			case kafkax.TopicTransferCompleted:
				var m bankv1.TransferCompleted
				if proto.Unmarshal(rec.Value, &m) == nil {
					h.add(event{Type: rec.Topic, TransferID: m.TransferId, FromAccountID: m.FromAccountId, ToAccountID: m.ToAccountId, AmountMinor: m.AmountMinor, OccurredAt: m.OccurredAt})
				}
			case kafkax.TopicTransferFailed:
				var m bankv1.TransferFailed
				if proto.Unmarshal(rec.Value, &m) == nil {
					h.add(event{Type: rec.Topic, TransferID: m.TransferId, FromAccountID: m.FromAccountId, ToAccountID: m.ToAccountId, AmountMinor: m.AmountMinor, Reason: m.Reason, OccurredAt: m.OccurredAt})
				}
			}
			slog.Info("consumed", "topic", rec.Topic, "partition", rec.Partition, "offset", rec.Offset, "key", string(rec.Key))
		}
		if err := cl.CommitUncommittedOffsets(ctx); err != nil { // commit only after processing
			slog.Error("commit", "err", err)
		}
	}
}
