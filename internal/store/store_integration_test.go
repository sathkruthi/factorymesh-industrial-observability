//go:build integration

package store

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sathkruthi/factorymesh-industrial-observability/internal/telemetry"
)

var schemaID atomic.Int64

func testStore(t *testing.T) *Store {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Fatal("DATABASE_URL required; integration tests must not silently skip")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("it_%d_%d", time.Now().UnixNano(), schemaID.Add(1))
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	c, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	c.ConnConfig.RuntimeParams["search_path"] = schema
	c.MaxConns = 8
	p, err := pgxpool.NewWithConfig(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	s := &Store{p, telemetry.Rule{High: 80000, Clear: 75000, Cooldown: time.Minute}}
	t.Cleanup(func() { p.Close(); admin.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE"); admin.Close() })
	if err = s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err = s.Migrate(ctx); err != nil {
		t.Fatal("migration not repeatable", err)
	}
	if err = s.Provision(ctx, "press-001", 1); err != nil {
		t.Fatal(err)
	}
	return s
}
func event(seq, temp int64) telemetry.Event {
	return telemetry.Event{Schema: 1, Device: "press-001", Generation: 1, Sequence: seq, ObservedAt: time.Unix(100+seq, 0), Temperature: temp, Correlation: fmt.Sprintf("test-%d", seq)}
}
func current(t *testing.T, s *Store) Device {
	t.Helper()
	ds, err := s.Devices(context.Background(), time.Now(), time.Minute)
	if err != nil || len(ds) != 1 {
		t.Fatalf("devices %v %v", ds, err)
	}
	return ds[0]
}

func TestIncident001Ordering(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	unsafe := os.Getenv("FM_UNSAFE_ORDERING") == "1"
	for _, seq := range []int64{3, 1, 2} {
		if _, err := s.ingest(ctx, event(seq, 70000+seq), time.Now(), unsafe); err != nil {
			t.Fatal(err)
		}
	}
	d := current(t, s)
	if d.Sequence != 3 || d.Temperature != 70003 {
		t.Fatalf("late telemetry corrupted current state: sequence=%d temperature=%d; expected 3 / 70003", d.Sequence, d.Temperature)
	}
}

func TestConcurrentPermutationsAndDuplicates(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	var wg sync.WaitGroup
	errs := make(chan error, 200)
	perm := rand.New(rand.NewSource(42)).Perm(100)
	for _, n := range perm {
		for repeat := 0; repeat < 2; repeat++ {
			wg.Add(1)
			go func(seq int64) { defer wg.Done(); _, err := s.Ingest(ctx, event(seq, 70000), time.Now()); errs <- err }(int64(n + 1))
		}
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if d := current(t, s); d.Sequence != 100 {
		t.Fatal("concurrent ordering failed", d.Sequence)
	}
	var count int
	if err := s.Pool.QueryRow(ctx, "SELECT count(*) FROM telemetry").Scan(&count); err != nil || count != 100 {
		t.Fatalf("duplicate persistence count=%d err=%v", count, err)
	}
}

func TestConflictingDuplicateAndOldGeneration(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	e := event(1, 80000)
	s.Ingest(ctx, e, time.Now())
	e.Temperature = 90000
	if _, err := s.Ingest(ctx, e, time.Now()); err != ErrConflict {
		t.Fatalf("conflict accepted: %v", err)
	}
	e.Generation = 2
	if _, err := s.Ingest(ctx, e, time.Now()); err != ErrStream {
		t.Fatalf("unregistered generation accepted: %v", err)
	}
	d := current(t, s)
	if d.Sequence != 1 || d.Temperature != 80000 {
		t.Fatal("rejected event mutated state")
	}
}

func TestRollbackOnAlertPersistenceFailure(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	if _, err := s.Pool.Exec(ctx, "ALTER TABLE alert_transitions ADD CONSTRAINT injected_failure CHECK (state <> 'firing')"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Ingest(ctx, event(1, 90000), time.Now()); err == nil {
		t.Fatal("injected constraint failed to reject alert")
	}
	d := current(t, s)
	var n int
	s.Pool.QueryRow(ctx, "SELECT count(*) FROM telemetry").Scan(&n)
	if d.Sequence != 0 || d.Active || n != 0 {
		t.Fatal("partial mutation after rollback", d, n)
	}
}

func TestIncident003AlertNoise(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	now := time.Unix(100, 0)
	temps := []int64{80001, 79999, 80001, 79999, 80001, 74000, 81000, 74000, 85000}
	for i, temp := range temps {
		at := now.Add(time.Duration(i) * time.Second)
		if i == 8 {
			at = now.Add(2 * time.Minute)
		}
		if _, err := s.Ingest(ctx, event(int64(i+1), temp), at); err != nil {
			t.Fatal(err)
		}
	}
	as, err := s.Alerts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	notices := 0
	for _, a := range as {
		if a.Notify {
			notices++
		}
	}
	if notices != 2 || len(as) != 5 || !current(t, s).Active {
		t.Fatalf("noise handling: notifications=%d transitions=%d", notices, len(as))
	}
}

func TestLateTelemetryDoesNotClearAlert(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	s.Ingest(ctx, event(2, 85000), time.Now())
	s.Ingest(ctx, event(1, 60000), time.Now())
	if !current(t, s).Active {
		t.Fatal("late event cleared live alert")
	}
}

func BenchmarkIngest(b *testing.B) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		b.Fatal("DATABASE_URL required")
	}
	ctx := context.Background()
	s, err := Open(ctx, url, telemetry.Rule{High: 80000, Clear: 75000, Cooldown: time.Minute})
	if err != nil {
		b.Fatal(err)
	}
	defer s.Pool.Close()
	id := fmt.Sprintf("bench-%d", time.Now().UnixNano())
	if err = s.Migrate(ctx); err != nil {
		b.Fatal(err)
	}
	if err = s.Provision(ctx, id, 1); err != nil {
		b.Fatal(err)
	}
	defer func() {
		s.Pool.Exec(ctx, "DELETE FROM telemetry WHERE device=$1", id)
		s.Pool.Exec(ctx, "DELETE FROM devices WHERE id=$1", id)
	}()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e := event(int64(i+1), 70000)
		e.Device = id
		if _, err = s.Ingest(ctx, e, time.Now()); err != nil {
			b.Fatal(err)
		}
	}
}
