package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sathkruthi/factorymesh-industrial-observability/internal/store"
	"github.com/sathkruthi/factorymesh-industrial-observability/internal/telemetry"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"
)

type fakeMessage struct {
	b   []byte
	ack atomic.Int32
}

func (m *fakeMessage) Payload() []byte { return m.b }
func (m *fakeMessage) Ack()            { m.ack.Add(1) }

type fakeSink struct {
	calls   atomic.Int32
	fail    atomic.Bool
	entered chan struct{}
	gate    chan struct{}
}

func (s *fakeSink) Ingest(ctx context.Context, e telemetry.Event, now time.Time) (store.Result, error) {
	s.calls.Add(1)
	if s.entered != nil {
		select {
		case s.entered <- struct{}{}:
		default:
		}
	}
	if s.gate != nil {
		select {
		case <-s.gate:
		case <-ctx.Done():
			return store.Result{}, ctx.Err()
		}
	}
	if s.fail.Load() {
		return store.Result{}, errors.New("outage")
	}
	return store.Result{Kind: "advanced"}, nil
}
func message(seq int64) *fakeMessage {
	b, _ := json.Marshal(telemetry.Event{Schema: 1, Device: "press-001", Generation: 1, Sequence: seq, ObservedAt: time.Unix(100, 0), Temperature: 70000, Correlation: "pipeline-test"})
	return &fakeMessage{b: b}
}
func await(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition not reached")
}

func TestAckOnlyAfterCommitAndRecovery(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := &fakeSink{}
	s.fail.Store(true)
	p := New(ctx, s, 2, 1, prometheus.NewRegistry(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	m := message(1)
	p.Submit(m)
	await(t, func() bool { return s.calls.Load() > 0 })
	if m.ack.Load() != 0 {
		t.Fatal("acknowledged before durable commit")
	}
	s.fail.Store(false)
	await(t, func() bool { return m.ack.Load() == 1 })
	cancel()
	p.Wait()
}

func TestBoundedAdmissionAndCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := &fakeSink{entered: make(chan struct{}, 1), gate: make(chan struct{})}
	p := New(ctx, s, 1, 1, prometheus.NewRegistry(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	m := message(1)
	p.Submit(m)
	<-s.entered
	p.Submit(message(2))
	blocked := make(chan bool, 1)
	go func() { blocked <- p.Submit(message(3)) }()
	select {
	case <-blocked:
		t.Fatal("overfull queue admitted event")
	case <-time.After(30 * time.Millisecond):
	}
	if len(p.queue) != 1 {
		t.Fatal("queue bound violated")
	}
	cancel()
	select {
	case accepted := <-blocked:
		if accepted {
			t.Fatal("cancelled admission succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("shutdown blocked")
	}
	p.Wait()
	if m.ack.Load() != 0 {
		t.Fatal("cancelled uncommitted work acknowledged")
	}
}

func TestMalformedMessageRejectedWithoutStorage(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := &fakeSink{}
	p := New(ctx, s, 1, 1, prometheus.NewRegistry(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	m := &fakeMessage{b: []byte(`{}`)}
	p.Submit(m)
	await(t, func() bool { return m.ack.Load() == 1 })
	cancel()
	p.Wait()
	if s.calls.Load() != 0 {
		t.Fatal("invalid payload reached database")
	}
}
