package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	DatabaseURL, BrokerURL, Listen string
	QueueCapacity, Workers         int
	High, Clear                    int64
	Cooldown, StaleAfter           time.Duration
}

func Load() (Config, error) { return Parse(os.Getenv) }

func Parse(env func(string) string) (Config, error) {
	c := Config{DatabaseURL: env("DATABASE_URL"), BrokerURL: env("BROKER_URL"), Listen: env("LISTEN_ADDR"), QueueCapacity: 64, Workers: 2, High: 80000, Clear: 75000, Cooldown: time.Minute, StaleAfter: 30 * time.Second}
	if c.DatabaseURL == "" {
		return c, fmt.Errorf("DATABASE_URL is required")
	}
	if c.BrokerURL == "" {
		c.BrokerURL = "tcp://broker:1883"
	}
	if c.Listen == "" {
		c.Listen = ":8080"
	}
	for key, target := range map[string]*int{"QUEUE_CAPACITY": &c.QueueCapacity, "WORKERS": &c.Workers} {
		if value := env(key); value != "" {
			n, err := strconv.Atoi(value)
			if err != nil {
				return c, fmt.Errorf("%s must be an integer", key)
			}
			*target = n
		}
	}
	for key, target := range map[string]*int64{"ALERT_HIGH_MC": &c.High, "ALERT_CLEAR_MC": &c.Clear} {
		if value := env(key); value != "" {
			n, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return c, fmt.Errorf("%s must be an integer", key)
			}
			*target = n
		}
	}
	for key, target := range map[string]*time.Duration{"ALERT_COOLDOWN": &c.Cooldown, "STALE_AFTER": &c.StaleAfter} {
		if value := env(key); value != "" {
			d, err := time.ParseDuration(value)
			if err != nil {
				return c, fmt.Errorf("%s must be a duration", key)
			}
			*target = d
		}
	}
	if c.QueueCapacity < 1 || c.QueueCapacity > 4096 {
		return c, fmt.Errorf("QUEUE_CAPACITY must be in [1,4096]")
	}
	if c.Workers < 1 || c.Workers > 32 {
		return c, fmt.Errorf("WORKERS must be in [1,32]")
	}
	if c.Clear < -40000 || c.High > 200000 || c.Clear >= c.High {
		return c, fmt.Errorf("alert thresholds require -40000 <= clear < high <= 200000")
	}
	if c.Cooldown <= 0 || c.StaleAfter <= 0 {
		return c, fmt.Errorf("durations must be positive")
	}
	return c, nil
}
