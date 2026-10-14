package main

import (
	"encoding/json"
	"flag"
	"fmt"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/sathkruthi/factorymesh-industrial-observability/internal/telemetry"
	"math/rand/v2"
	"os"
	"time"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	broker := flag.String("broker", "tcp://broker:1883", "broker URL")
	device := flag.String("device", "press-001", "provisioned device")
	count := flag.Int("count", 30, "event count (1-100000)")
	start := flag.Int64("start", 1, "first sequence; reuse only for identical replay fixtures")
	generation := flag.Int64("generation", 1, "provisioned generation")
	interval := flag.Duration("interval", 100*time.Millisecond, "event interval")
	duplicate := flag.Bool("duplicate", false, "publish each event twice with identical content")
	flag.Parse()
	if *count < 1 || *count > 100000 || *interval < 0 || *start < 1 {
		return fmt.Errorf("invalid count, sequence or interval")
	}
	client := mqtt.NewClient(mqtt.NewClientOptions().AddBroker(*broker).SetClientID(fmt.Sprintf("factorymesh-simulator-%d", time.Now().UnixNano())).SetConnectTimeout(5 * time.Second).SetWriteTimeout(5 * time.Second))
	t := client.Connect()
	if !t.WaitTimeout(8 * time.Second) {
		return fmt.Errorf("broker connect timed out")
	}
	if t.Error() != nil {
		return t.Error()
	}
	defer client.Disconnect(250)
	for i := 0; i < *count; i++ {
		e := telemetry.Event{Schema: 1, Device: *device, Generation: *generation, Sequence: *start + int64(i), ObservedAt: time.Now().UTC(), Temperature: 70000 + rand.Int64N(16000), Correlation: fmt.Sprintf("sim-%d-%d", *start, i)}
		if err := e.Validate(); err != nil {
			return err
		}
		b, _ := json.Marshal(e)
		repeat := 1
		if *duplicate {
			repeat = 2
		}
		for j := 0; j < repeat; j++ {
			t = client.Publish("factorymesh/telemetry", 1, false, b)
			if !t.WaitTimeout(8 * time.Second) {
				return fmt.Errorf("publish timeout at sequence %d: delivery uncertain", e.Sequence)
			}
			if t.Error() != nil {
				return t.Error()
			}
		}
		fmt.Println(string(b))
		time.Sleep(*interval)
	}
	return nil
}
