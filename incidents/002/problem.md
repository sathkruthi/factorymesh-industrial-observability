# INCIDENT-002: Gateway failure loses acknowledged work

Synthetic engineering exercise; not a historical production outage.

A database outage overlaps telemetry receipt. A naive receipt-time acknowledgement removes broker responsibility before durable commit, making a gateway crash capable of losing the measurement.

Environment: local Go service, PostgreSQL, and MQTT. Workload and evidence contain synthetic equipment IDs only. Preserve ordering, database atomicity, and documented retention limits.
