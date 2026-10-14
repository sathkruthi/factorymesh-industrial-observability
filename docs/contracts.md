# Contracts

## Telemetry JSON

```json
{"schema":1,"device":"press-001","generation":1,"sequence":1,"observed_at":"2026-01-01T00:00:00Z","temperature_mc":80000,"correlation_id":"demo-001"}
```

All fields are required and non-null; unknown fields and trailing JSON values are rejected. Payload limit: 2048 bytes. Device and correlation IDs: 1–64 letters, digits, underscores, or hyphens. Positive generation and sequence. Temperature: -40000 to 200000 milli-degrees. Event timestamps are metadata, never the ordering key. Publisher clocks need not agree.

Topic: `factorymesh/telemetry`, QoS 1. Replays must retain content, including correlation ID and timestamp. Canonical event fingerprints normalize timestamps to UTC. State ordering applies within the explicitly registered generation; clients cannot reset it by sending a different generation.

A delayed measurement persists but cannot overwrite newer state or clear its alert. Concurrent ingestion may process higher sequences first; short-lived threshold excursions arriving late are retained but not evaluated as current-state alerts. The system is a state monitor, not an exhaustive historical anomaly detector.

## State and alerts

Connectivity uses ingestion time, not device clock. An accepted replay can refresh connectivity while preserving the measurement. `unknown` means no accepted receipt; `stale` means the configured inactivity threshold elapsed. `healthy` means recently received telemetry, not independently certified equipment health.

Hysteresis: inactive fires at `temperature >= high`; active clears at `temperature <= clear`. Clear must be lower than high. Cooldown affects the `notify` flag on new firing transitions, never suppression of state transitions. Clear transitions are always retained. No notification transport exists in this release.

Read APIs: `/health/live`, `/health/ready`, `/devices`, `/devices/{id}`, `/alerts` (latest 100 transitions), `/metrics`. Readiness fails on database or MQTT subscription unavailability. Liveness remains healthy during dependency outages.

## Configuration

| Variable | Default | Contract |
| --- | --- | --- |
| DATABASE_URL | required | pgx connection URL; runtime environment only |
| BROKER_URL | tcp://broker:1883 | trusted local MQTT broker |
| LISTEN_ADDR | :8080 | Compose publishes only on loopback |
| QUEUE_CAPACITY | 64 | 1–4096 waiting messages |
| WORKERS | 2 | 1–32 processing workers |
| ALERT_HIGH_MC | 80000 | above clear, <=200000 |
| ALERT_CLEAR_MC | 75000 | >=-40000 and below high |
| ALERT_COOLDOWN | 1m | positive Go duration |
| STALE_AFTER | 30s | positive Go duration |
| OTEL_EXPORTER_OTLP_ENDPOINT | unset | optional HTTP trace exporter endpoint |

## Delivery boundaries

Paho automatic acknowledgement is disabled. Worker commit precedes message acknowledgement. The persistent subscriber ID is fixed; this release supports one ingestion service replica only. Mosquitto limits outbound in-flight messages to 16, queued messages to 2000, queued bytes to 4096000, and payload size to 2048 bytes. Queue overflow can discard broker messages even when a publisher received an acknowledgement. Broker autosave is not fsync-per-publish durability. Never claim lossless recovery across arbitrary crashes.

Invalid schema, unknown stream, and conflicting replay are acknowledged after explicit rejection to prevent poison-message retry loops. Rejections appear in logs, counters, and spans where applicable; no durable rejection quarantine is implemented. Database failures are retried indefinitely with per-attempt timeouts and delay capped near 6.25 seconds including jitter. Permanent SQL configuration errors require operator repair; the service does not acknowledge away such failures.
