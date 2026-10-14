package telemetry

import (
	"encoding/json"
	"testing"
	"time"
)

func Sample(seq, temp int64) Event {
	return Event{1, "press-001", 1, seq, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), temp, "sample"}
}

func TestDecode(t *testing.T) {
	e := Sample(1, 80000)
	b, _ := json.Marshal(e)
	got, err := Decode(b)
	if err != nil || got != e {
		t.Fatalf("round trip %v %v", got, err)
	}
	for _, p := range [][]byte{nil, []byte(`{}`), append(b, []byte(` {}`)...), []byte(`{"schema":1,"surprise":1}`), make([]byte, MaxPayload+1)} {
		if _, err := Decode(p); err == nil {
			t.Fatalf("invalid input accepted %q", p)
		}
	}
}

func TestAlertHysteresisCooldown(t *testing.T) {
	r := Rule{80000, 75000, time.Minute}
	now := time.Unix(100, 0)
	d := r.Evaluate(Alert{}, 80000, now)
	if !d.Notify || !d.Next.Active {
		t.Fatal("first firing suppressed")
	}
	for _, temp := range []int64{79999, 80001, 76000} {
		d = r.Evaluate(d.Next, temp, now.Add(time.Second))
		if d.Transition != "" || d.Notify || !d.Next.Active {
			t.Fatal("noise changed firing state")
		}
	}
	d = r.Evaluate(d.Next, 75000, now.Add(2*time.Second))
	if d.Next.Active || d.Transition != "cleared" {
		t.Fatal("clear threshold failed")
	}
	d = r.Evaluate(d.Next, 80000, now.Add(3*time.Second))
	if !d.Next.Active || d.Notify {
		t.Fatal("cooldown must suppress notice without suppressing state")
	}
	d = r.Evaluate(d.Next, 74000, now.Add(time.Minute))
	d = r.Evaluate(d.Next, 85000, now.Add(2*time.Minute))
	if !d.Notify {
		t.Fatal("meaningful later alert suppressed")
	}
}

func FuzzDecode(f *testing.F) {
	b, _ := json.Marshal(Sample(1, 80000))
	f.Add(b)
	f.Add([]byte(`{}`))
	f.Fuzz(func(t *testing.T, b []byte) {
		e, err := Decode(b)
		if err == nil {
			if err = e.Validate(); err != nil {
				t.Fatal(err)
			}
		}
	})
}
