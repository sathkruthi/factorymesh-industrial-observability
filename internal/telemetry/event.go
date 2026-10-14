package telemetry

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"time"
)

const MaxPayload = 2048

var identifier = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

type Event struct {
	Schema      int       `json:"schema"`
	Device      string    `json:"device"`
	Generation  int64     `json:"generation"`
	Sequence    int64     `json:"sequence"`
	ObservedAt  time.Time `json:"observed_at"`
	Temperature int64     `json:"temperature_mc"`
	Correlation string    `json:"correlation_id"`
}

func Decode(payload []byte) (Event, error) {
	var e Event
	if len(payload) > MaxPayload {
		return e, fmt.Errorf("payload exceeds %d bytes", MaxPayload)
	}
	d := json.NewDecoder(bytes.NewReader(payload))
	d.DisallowUnknownFields()
	if err := d.Decode(&e); err != nil {
		return e, fmt.Errorf("invalid JSON: %w", err)
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return e, fmt.Errorf("payload must contain one JSON value")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(payload, &fields); err != nil {
		return e, err
	}
	for _, key := range []string{"schema", "device", "generation", "sequence", "observed_at", "temperature_mc", "correlation_id"} {
		v, ok := fields[key]
		if !ok || bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
			return e, fmt.Errorf("%s is required and cannot be null", key)
		}
	}
	return e, e.Validate()
}

func (e Event) Validate() error {
	if e.Schema != 1 {
		return fmt.Errorf("schema must equal 1")
	}
	if !identifier.MatchString(e.Device) || !identifier.MatchString(e.Correlation) {
		return fmt.Errorf("device and correlation identifiers must be 1-64 alphanumeric, underscore or hyphen characters")
	}
	if e.Generation < 1 || e.Sequence < 1 {
		return fmt.Errorf("generation and sequence must be positive")
	}
	if e.ObservedAt.IsZero() {
		return fmt.Errorf("observed_at required")
	}
	if e.Temperature < -40000 || e.Temperature > 200000 {
		return fmt.Errorf("temperature outside sensor range")
	}
	return nil
}

// Fingerprint includes correlation metadata: replay must preserve the original event.
func (e Event) Fingerprint() []byte {
	e.ObservedAt = e.ObservedAt.UTC()
	b, _ := json.Marshal(e)
	h := sha256.Sum256(b)
	return h[:]
}

type Alert struct {
	Active     bool
	LastNotice time.Time
}
type Decision struct {
	Next       Alert
	Transition string
	Notify     bool
}
type Rule struct {
	High, Clear int64
	Cooldown    time.Duration
}

func (r Rule) Evaluate(prior Alert, temp int64, now time.Time) Decision {
	d := Decision{Next: prior}
	if !prior.Active && temp >= r.High {
		d.Next.Active = true
		d.Transition = "firing"
	}
	if prior.Active && temp <= r.Clear {
		d.Next.Active = false
		d.Transition = "cleared"
	}
	if d.Transition == "firing" && (prior.LastNotice.IsZero() || now.Sub(prior.LastNotice) >= r.Cooldown) {
		d.Notify = true
		d.Next.LastNotice = now
	}
	return d
}
