package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sathkruthi/factorymesh-industrial-observability/internal/config"
	"github.com/sathkruthi/factorymesh-industrial-observability/internal/pipeline"
	"github.com/sathkruthi/factorymesh-industrial-observability/internal/store"
	"github.com/sathkruthi/factorymesh-industrial-observability/internal/telemetry"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	trace "go.opentelemetry.io/otel/sdk/trace"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		log.Error("service stopped", "reason", err.Error())
		os.Exit(1)
	}
}
func run(log *slog.Logger) error {
	c, err := config.Load()
	if err != nil {
		return err
	}
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") != "" {
		exp, err := otlptracehttp.New(ctx)
		if err != nil {
			return fmt.Errorf("trace exporter: %w", err)
		}
		tp := trace.NewTracerProvider(trace.WithBatcher(exp), trace.WithResource(resource.NewWithAttributes("", attribute.String("service.name", "factorymesh"))))
		otel.SetTracerProvider(tp)
		defer func() {
			end, done := context.WithTimeout(context.Background(), 3*time.Second)
			defer done()
			tp.Shutdown(end)
		}()
	}
	db, err := store.Open(ctx, c.DatabaseURL, telemetry.Rule{High: c.High, Clear: c.Clear, Cooldown: c.Cooldown})
	if err != nil {
		return err
	}
	defer db.Pool.Close()
	startup, done := context.WithTimeout(ctx, 10*time.Second)
	defer done()
	if err = db.Migrate(startup); err != nil {
		return err
	}
	for i := 1; i <= 3; i++ {
		if err = db.Provision(startup, fmt.Sprintf("press-%03d", i), 1); err != nil {
			return err
		}
	}
	reg := prometheus.NewRegistry()
	reg.MustRegister(prometheus.NewGoCollector(), prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
	p := pipeline.New(ctx, db, c.QueueCapacity, c.Workers, reg, log)
	defer p.Wait()
	var subscribed atomic.Bool
	opts := mqtt.NewClientOptions().AddBroker(c.BrokerURL).SetClientID("factorymesh-ingestion-v1").SetCleanSession(false).SetAutoReconnect(true).SetConnectRetry(true).SetAutoAckDisabled(true).SetOrderMatters(true).SetKeepAlive(15 * time.Second).SetPingTimeout(5 * time.Second).SetConnectTimeout(3 * time.Second).SetResumeSubs(true)
	opts.SetDefaultPublishHandler(p.Handle)
	opts.SetOnConnectHandler(func(client mqtt.Client) {
		subscribed.Store(false)
		token := client.Subscribe("factorymesh/telemetry", 1, p.Handle)
		if token.WaitTimeout(5*time.Second) && token.Error() == nil {
			subscribed.Store(true)
			log.Info("broker subscription ready")
		} else {
			log.Warn("broker subscription unavailable")
		}
	})
	opts.SetConnectionLostHandler(func(_ mqtt.Client, _ error) { subscribed.Store(false); log.Warn("broker connection lost") })
	client := mqtt.NewClient(opts)
	client.Connect()
	defer client.Disconnect(250)
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	mux.HandleFunc("GET /health/live", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK); w.Write([]byte("live\n")) })
	mux.HandleFunc("GET /health/ready", func(w http.ResponseWriter, r *http.Request) {
		op, end := context.WithTimeout(r.Context(), 2*time.Second)
		defer end()
		if !subscribed.Load() || !client.IsConnectionOpen() || db.Pool.Ping(op) != nil {
			http.Error(w, "dependency unavailable", 503)
			return
		}
		w.Write([]byte("ready\n"))
	})
	devices := func(w http.ResponseWriter, r *http.Request) {
		op, end := context.WithTimeout(r.Context(), 3*time.Second)
		defer end()
		ds, err := db.Devices(op, time.Now(), c.StaleAfter)
		if err != nil {
			http.Error(w, "database unavailable", 503)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if id := r.PathValue("id"); id != "" {
			for _, d := range ds {
				if d.ID == id {
					json.NewEncoder(w).Encode(d)
					return
				}
			}
			http.Error(w, "device not found", 404)
			return
		}
		json.NewEncoder(w).Encode(ds)
	}
	mux.HandleFunc("GET /devices", devices)
	mux.HandleFunc("GET /devices/{id}", devices)
	mux.HandleFunc("GET /alerts", func(w http.ResponseWriter, r *http.Request) {
		op, end := context.WithTimeout(r.Context(), 3*time.Second)
		defer end()
		as, err := db.Alerts(op)
		if err != nil {
			http.Error(w, "database unavailable", 503)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(as)
	})
	server := &http.Server{Addr: c.Listen, Handler: mux, ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16384}
	errs := make(chan error, 1)
	go func() { errs <- server.ListenAndServe() }()
	log.Info("service started", "queue_capacity", c.QueueCapacity, "workers", c.Workers)
	select {
	case <-ctx.Done():
	case err = <-errs:
		cancel()
		if err != http.ErrServerClosed {
			return err
		}
	}
	shutdown, end := context.WithTimeout(context.Background(), 5*time.Second)
	defer end()
	log.Info("shutdown: unacknowledged work will be replayed within broker retention limits")
	return server.Shutdown(shutdown)
}
