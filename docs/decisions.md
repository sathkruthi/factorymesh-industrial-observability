# Architecture decisions

## One service and PostgreSQL transactions

One process owns state and alert evaluation. Per-device row locks serialize concurrent events, and one transaction controls all durable effects. This avoids coordination across unnecessary microservices. A hot device is intentionally serialized and may limit throughput. Different devices can use the bounded worker pool.

## Explicit sequences over timestamps

Device time is unreliable for ordering. Registered generation plus monotonic sequence defines current state. The first release has three registered lab devices and no automatic generation reset. Late measurements remain visible without modifying current-state alerts.

## MQTT manual acknowledgements

Paho 1.5.1 exposes `SetAutoAckDisabled(true)` and `Message.Ack()`. The application disables receipt-time acknowledgement and calls Ack after commit. In-flight and queue limits combine application bounds with broker flow control. Durable database effects remain idempotent when an acknowledgement is uncertain. Broker receipt alone is not an end-to-end guarantee.

## Hysteresis and cooldown

A separate lower clear threshold avoids oscillation. Cooldown controls eligible notifications, while durable state transitions remain accurate. Transition history is inspection data; no external notification delivery is advertised.

## Local observability

Prometheus, Grafana, and Jaeger expose real runtime evidence. Metric labels use fixed disposition categories; device and correlation IDs belong in logs/traces to avoid unbounded metric cardinality. Tracing is optional; trace-exporter failure cannot block business persistence.
