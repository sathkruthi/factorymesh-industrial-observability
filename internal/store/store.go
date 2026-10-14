package store

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sathkruthi/factorymesh-industrial-observability/internal/telemetry"
)

//go:embed schema.sql
var schema string
var ErrConflict = errors.New("conflicting duplicate event")
var ErrStream = errors.New("unregistered device or generation")

type Store struct {
	Pool *pgxpool.Pool
	Rule telemetry.Rule
}
type Result struct {
	Kind       string
	Transition string
	Notify     bool
}
type Device struct {
	ID          string     `json:"id"`
	Generation  int64      `json:"generation"`
	Sequence    int64      `json:"sequence"`
	Temperature int64      `json:"temperature_mc"`
	ObservedAt  *time.Time `json:"observed_at"`
	LastSeen    *time.Time `json:"last_seen"`
	Active      bool       `json:"alert_active"`
	LastNotice  *time.Time `json:"last_notice"`
	Health      string     `json:"health"`
}
type Transition struct {
	ID          int64     `json:"id"`
	Device      string    `json:"device"`
	Sequence    int64     `json:"sequence"`
	State       string    `json:"state"`
	Notify      bool      `json:"notify"`
	At          time.Time `json:"at"`
	Correlation string    `json:"correlation_id"`
}

func Open(ctx context.Context, url string, rule telemetry.Rule) (*Store, error) {
	c, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, err
	}
	c.MaxConns = 8
	c.ConnConfig.ConnectTimeout = 3 * time.Second
	p, err := pgxpool.NewWithConfig(ctx, c)
	if err != nil {
		return nil, err
	}
	return &Store{p, rule}, nil
}
func (s *Store) Migrate(ctx context.Context) error { _, err := s.Pool.Exec(ctx, schema); return err }
func (s *Store) Provision(ctx context.Context, id string, generation int64) error {
	if err := (telemetry.Event{Schema: 1, Device: id, Generation: generation, Sequence: 1, ObservedAt: time.Now(), Correlation: "provision"}).Validate(); err != nil {
		return err
	}
	_, err := s.Pool.Exec(ctx, "INSERT INTO devices(id,generation) VALUES($1,$2) ON CONFLICT(id) DO NOTHING", id, generation)
	return err
}

func (s *Store) Ingest(ctx context.Context, e telemetry.Event, now time.Time) (Result, error) {
	return s.ingest(ctx, e, now, false)
}

func (s *Store) ingest(ctx context.Context, e telemetry.Event, now time.Time, unsafeOrdering bool) (Result, error) {
	if err := e.Validate(); err != nil {
		return Result{}, err
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Result{}, err
	}
	defer tx.Rollback(context.Background())
	var generation, sequence int64
	var active bool
	var notice *time.Time
	err = tx.QueryRow(ctx, "SELECT generation,sequence,alert_active,last_notice FROM devices WHERE id=$1 FOR UPDATE", e.Device).Scan(&generation, &sequence, &active, &notice)
	if errors.Is(err, pgx.ErrNoRows) {
		return Result{}, ErrStream
	}
	if err != nil {
		return Result{}, err
	}
	if generation != e.Generation {
		return Result{}, ErrStream
	}
	var old []byte
	err = tx.QueryRow(ctx, "SELECT fingerprint FROM telemetry WHERE device=$1 AND generation=$2 AND sequence=$3", e.Device, e.Generation, e.Sequence).Scan(&old)
	if err == nil {
		if !bytes.Equal(old, e.Fingerprint()) {
			return Result{}, ErrConflict
		}
		_, err = tx.Exec(ctx, "UPDATE devices SET last_seen=GREATEST(last_seen,$2) WHERE id=$1", e.Device, now)
		if err != nil {
			return Result{}, err
		}
		return Result{Kind: "duplicate"}, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Result{}, err
	}
	_, err = tx.Exec(ctx, "INSERT INTO telemetry(device,generation,sequence,fingerprint,temperature_mc,observed_at,received_at,correlation_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8)", e.Device, e.Generation, e.Sequence, e.Fingerprint(), e.Temperature, e.ObservedAt, now, e.Correlation)
	if err != nil {
		return Result{}, err
	}
	result := Result{Kind: "late"}
	if e.Sequence > sequence || unsafeOrdering {
		prior := telemetry.Alert{Active: active}
		if notice != nil {
			prior.LastNotice = *notice
		}
		decision := s.Rule.Evaluate(prior, e.Temperature, now)
		var nextNotice *time.Time
		if !decision.Next.LastNotice.IsZero() {
			n := decision.Next.LastNotice
			nextNotice = &n
		}
		_, err = tx.Exec(ctx, "UPDATE devices SET sequence=$2,temperature_mc=$3,observed_at=$4,last_seen=GREATEST(last_seen,$5),alert_active=$6,last_notice=$7 WHERE id=$1", e.Device, e.Sequence, e.Temperature, e.ObservedAt, now, decision.Next.Active, nextNotice)
		result = Result{Kind: "advanced", Transition: decision.Transition, Notify: decision.Notify}
		if err == nil && decision.Transition != "" {
			_, err = tx.Exec(ctx, "INSERT INTO alert_transitions(device,sequence,state,notify,created_at,correlation_id) VALUES($1,$2,$3,$4,$5,$6)", e.Device, e.Sequence, decision.Transition, decision.Notify, now, e.Correlation)
		}
	} else {
		_, err = tx.Exec(ctx, "UPDATE devices SET last_seen=GREATEST(last_seen,$2) WHERE id=$1", e.Device, now)
	}
	if err != nil {
		return Result{}, err
	}
	return result, tx.Commit(ctx)
}

func (s *Store) Devices(ctx context.Context, now time.Time, stale time.Duration) ([]Device, error) {
	rows, err := s.Pool.Query(ctx, "SELECT id,generation,sequence,temperature_mc,observed_at,last_seen,alert_active,last_notice FROM devices ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Device{}
	for rows.Next() {
		var d Device
		if err = rows.Scan(&d.ID, &d.Generation, &d.Sequence, &d.Temperature, &d.ObservedAt, &d.LastSeen, &d.Active, &d.LastNotice); err != nil {
			return nil, err
		}
		d.Health = "unknown"
		if d.LastSeen != nil {
			d.Health = "healthy"
			if now.Sub(*d.LastSeen) > stale {
				d.Health = "stale"
			}
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
func (s *Store) Alerts(ctx context.Context) ([]Transition, error) {
	rows, err := s.Pool.Query(ctx, "SELECT id,device,sequence,state,notify,created_at,correlation_id FROM alert_transitions ORDER BY id DESC LIMIT 100")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Transition{}
	for rows.Next() {
		var a Transition
		if err = rows.Scan(&a.ID, &a.Device, &a.Sequence, &a.State, &a.Notify, &a.At, &a.Correlation); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
func (s *Store) String() string {
	return fmt.Sprintf("FactoryMesh store (%d connections)", s.Pool.Config().MaxConns)
}
