# INCIDENT-001: Delayed telemetry corrupts equipment state

Synthetic engineering exercise; not a historical production outage.

The gateway receives sequences 3, 1, 2; a last-arrival-wins update regresses the visible state to sequence 2. A reviewer must distinguish device time from ingestion order.

Environment: local Go service, PostgreSQL, and MQTT. Workload and evidence contain synthetic equipment IDs only. Preserve ordering, database atomicity, and documented retention limits.
