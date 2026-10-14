package pipeline

import (
	"context"
	"errors"
	"log/slog"
	"math/rand/v2"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sathkruthi/factorymesh-industrial-observability/internal/store"
	"github.com/sathkruthi/factorymesh-industrial-observability/internal/telemetry"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

type Sink interface {
	Ingest(context.Context, telemetry.Event, time.Time) (store.Result, error)
}
type Message interface {
	Payload() []byte
	Ack()
}
type Pipeline struct {
	ctx          context.Context
	sink         Sink
	queue        chan Message
	wg           sync.WaitGroup
	log          *slog.Logger
	Events       *prometheus.CounterVec
	Retries      prometheus.Counter
	Latency      prometheus.Histogram
	Queue        prometheus.Gauge
	Backpressure prometheus.Counter
}

func New(ctx context.Context, sink Sink, capacity, workers int, reg prometheus.Registerer, log *slog.Logger) *Pipeline {
	p := &Pipeline{ctx: ctx, sink: sink, queue: make(chan Message, capacity), log: log,
		Events:       prometheus.NewCounterVec(prometheus.CounterOpts{Name: "factorymesh_events_total", Help: "Telemetry disposition."}, []string{"result"}),
		Retries:      prometheus.NewCounter(prometheus.CounterOpts{Name: "factorymesh_retries_total", Help: "Database retry attempts."}),
		Latency:      prometheus.NewHistogram(prometheus.HistogramOpts{Name: "factorymesh_processing_seconds", Help: "Processing time including database retries.", Buckets: prometheus.DefBuckets}),
		Queue:        prometheus.NewGauge(prometheus.GaugeOpts{Name: "factorymesh_queue_depth", Help: "Current bounded queue occupancy."}),
		Backpressure: prometheus.NewCounter(prometheus.CounterOpts{Name: "factorymesh_backpressure_total", Help: "Queue admission encountered full capacity."})}
	reg.MustRegister(p.Events, p.Retries, p.Latency, p.Queue, p.Backpressure)
	for i := 0; i < workers; i++ {
		p.wg.Add(1)
		go p.worker()
	}
	return p
}

func (p *Pipeline) Handle(_ mqtt.Client, m mqtt.Message) { p.Submit(m) }
func (p *Pipeline) Submit(m Message) bool {
	if p.ctx.Err() != nil {
		return false
	}
	select {
	case p.queue <- m:
		p.Queue.Set(float64(len(p.queue)))
		return true
	default:
		p.Backpressure.Inc()
	}
	select {
	case p.queue <- m:
		p.Queue.Set(float64(len(p.queue)))
		return true
	case <-p.ctx.Done():
		return false
	}
}
func (p *Pipeline) Wait() { p.wg.Wait() }

func (p *Pipeline) worker() {
	defer p.wg.Done()
	for {
		select {
		case <-p.ctx.Done():
			return
		case m := <-p.queue:
			p.Queue.Set(float64(len(p.queue)))
			p.process(m)
		}
	}
}
func (p *Pipeline) process(m Message) {
	start := time.Now()
	defer func() { p.Latency.Observe(time.Since(start).Seconds()) }()
	e, err := telemetry.Decode(m.Payload())
	if err != nil {
		p.Events.WithLabelValues("invalid").Inc()
		p.log.Warn("telemetry rejected", "reason", err.Error())
		m.Ack()
		return
	}
	ctx, span := otel.Tracer("factorymesh/ingest").Start(p.ctx, "telemetry.ingest")
	defer span.End()
	span.SetAttributes(attribute.String("device.id", e.Device), attribute.String("correlation.id", e.Correlation), attribute.Int64("sequence", e.Sequence))
	for attempt := 0; ; attempt++ {
		if ctx.Err() != nil {
			return
		}
		op, cancel := context.WithTimeout(ctx, 5*time.Second)
		result, err := p.sink.Ingest(op, e, time.Now().UTC())
		cancel()
		if err == nil {
			p.Events.WithLabelValues(result.Kind).Inc()
			p.log.Info("telemetry committed", "device", e.Device, "sequence", e.Sequence, "correlation_id", e.Correlation, "result", result.Kind, "alert_transition", result.Transition, "notify", result.Notify)
			m.Ack()
			return
		}
		if errors.Is(err, store.ErrConflict) || errors.Is(err, store.ErrStream) {
			p.Events.WithLabelValues("rejected").Inc()
			p.log.Warn("telemetry rejected", "device", e.Device, "correlation_id", e.Correlation, "reason", err.Error())
			span.SetStatus(codes.Error, err.Error())
			m.Ack()
			return
		}
		p.Retries.Inc()
		p.log.Warn("database retry", "device", e.Device, "correlation_id", e.Correlation, "attempt", attempt+1)
		delay := 100 * time.Millisecond
		for i := 0; i < attempt && delay < 5*time.Second; i++ {
			delay *= 2
		}
		if delay > 5*time.Second {
			delay = 5 * time.Second
		}
		delay += time.Duration(rand.Int64N(int64(delay/4) + 1))
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}
