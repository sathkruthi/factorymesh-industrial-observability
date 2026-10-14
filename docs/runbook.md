# Operations

1. Check `curl -i http://127.0.0.1:8080/health/ready` and `/health/live`.
2. Inspect `docker compose ps` and `docker compose logs --tail=100 api database broker`.
3. Find a telemetry correlation ID in the JSON service logs and corresponding `telemetry.ingest` spans in Jaeger.
4. Inspect queue occupancy, retry count, goroutines, and resident memory in the Grafana dashboard.
5. Query `/devices/{id}` and `/alerts`; compare current sequence with the incoming fixture.

Database unavailable: readiness 503, liveness 200, commit-before-ack workers retry, MQTT broker holds pending traffic up to its configured retention limits. Restore the database and watch retries cease and current sequence advance. Do not restart everything before capturing evidence.

Broker unavailable: readiness 503 and a `broker connection lost` log. The client reconnects and subscribes again; confirm `broker subscription ready`. Producers must preserve their original event when retrying an uncertain publication. The sample simulator exits on an uncertain publish rather than silently inventing a replacement.

Conflicting duplicates: compare the complete payload, including correlation ID and event timestamp. A fresh simulator run reusing old sequences is a conflict, not a replay. `make demo` selects a new start sequence. Stream rotation is not implemented; do not change a device generation while ingesting.

Port conflict: edit the host side of the corresponding `127.0.0.1:host:container` mapping. Also adjust local test URLs and scripts when changing ports. Do not stop an unrelated application to free a port.

Retention: events persist in the named PostgreSQL volume until manually removed by the owner. No automated deletion or archival exists. Grafana and Jaeger demonstration data are ephemeral. PostgreSQL migrations run at startup using idempotent initial DDL; schema evolution needs a versioned migration mechanism in a future release.

Shutdown: `make down` retains named data volumes. Workers cancel rather than draining indefinitely; acknowledgement follows commit, so cancelled uncommitted events can be replayed within broker limits.
