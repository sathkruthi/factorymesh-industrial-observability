# INCIDENT-003: Noisy sensor floods eligible alert notifications

Synthetic engineering exercise; not a historical production outage.

Temperatures oscillate around 80000 milli-degrees. A single threshold with no cooldown repeatedly fires and clears notifications despite no meaningful recovery.

Environment: local Go service, PostgreSQL, and MQTT. Workload and evidence contain synthetic equipment IDs only. Preserve ordering, database atomicity, and documented retention limits.
