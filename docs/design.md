# FactoryMesh — implementation design

Status: initial pre-implementation proposal preserved for traceability. The implementation contracts and measured results are documented in contracts.md and verification.md. The review records scope adjustments.

## Problem and scope

Ingest synthetic industrial equipment telemetry without letting duplicate or delayed events corrupt the current device state. Provide observable, reproducible evidence for ordering failures, connectivity recovery, and noisy alert behavior. Single-host Docker Compose is the first complete release; no real equipment, customer data, or cloud deployment.

## Components

Go simulator → MQTT broker → Go ingestion service → PostgreSQL.

The service exposes read-only device state and health APIs, Prometheus metrics, structured logs, and OpenTelemetry spans. Grafana consumes metrics; a local trace collector receives spans. One backend process owns validation, persistence, and alert evaluation to keep transaction boundaries explicit.

MQTT demonstrates reconnect and retry behavior. PostgreSQL provides durable transactions and uniqueness constraints. A bounded work queue limits memory; broker persistence and acknowledgement semantics must be tested rather than assumed.

## Contracts and data

Telemetry JSON: schema version, device ID, stream generation, sequence, observed timestamp, integer temperature in milli-degrees Celsius, and correlation ID. Reject unknown schema versions, oversized payloads, invalid identifiers, negative sequences, and implausible sensor ranges. Use synthetic device IDs only.

Tables: registered device stream generations; immutable accepted telemetry keyed by device/generation/sequence; current device state; alert state; alert transitions. The first release uses explicitly provisioned stream generations, preventing an arbitrary client from resetting a sequence. Device state advances only within the registered generation and for a higher sequence. A generation change requires an explicit administrative operation; delayed records from previous generations cannot update state.

Duplicate with identical content: successful no-op. Same key with conflicting content: reject and count a conflict. Persist valid late events for investigation, without changing state or reevaluating current-state alerts. Use a transaction and per-device locking to make insertion, state advancement, and alert transitions atomic. Retain both event time and ingestion time; connectivity is based on ingestion time.

Read APIs: `GET /health/live`, `GET /health/ready`, `GET /devices`, `GET /devices/{id}`, `GET /alerts`, `GET /metrics`. Readiness requires database access and ingestion connectivity. Liveness must not fail merely because a dependency is temporarily unavailable. No public deployment; bind local demonstration endpoints to loopback.

## State and recovery

Connectivity: unknown → healthy after an accepted receipt → stale after a configurable inactivity interval → healthy after recovery. Duplicate receipts may demonstrate connectivity but cannot alter the last accepted measurement.

Alerts: normal → firing at the high threshold → normal below a lower clear threshold. Cooldown suppresses repeated notifications, not state changes. Evaluate only newly advanced device state; use a controllable clock for deterministic tests. Alert transitions persist inside the same transaction as state updates.

Queue capacity, payload limit, worker count, connection limits, retry ceilings, and timeouts are explicit configuration. Full queue must have a documented rejection or broker backpressure contract; never silently discard an acknowledged event. Verify the MQTT library's acknowledgment timing before choosing its handler/worker design. Retry transient database errors with bounded exponential backoff and jitter. Permanent validation failures are rejected with a reason. Shutdown stops admission, drains within a deadline, and reports undrained work.

Delivery guarantee: at-least-once only across boundaries demonstrated by tests, with idempotent database effects. No exactly-once claim. Broker acknowledgement is not equivalent to a committed database write. Explicitly document any broker-to-database crash window discovered during implementation.

## Configuration and security

Validate configuration before opening listeners. Read database and broker credentials from environment variables; tracked examples contain development-only placeholders. Avoid raw credentials and entire payloads in logs. Parameterized SQL, payload size limits, request timeouts, loopback exposure, least-privilege CI, and restricted database roles. Synthetic data only. Threat boundary: trusted local broker publishers in this release; production device authentication is outside scope and must be stated.

## Verification and incidents

Unit: schema rejection, duplicate/conflict classification, ordering, generation changes, alert hysteresis, cooldown, retry limits, configuration bounds.

Database integration: concurrent state updates, transaction rollback, uniqueness conflict behavior, restart persistence, and migration repeatability. MQTT integration: disconnect/reconnect, broker restart, replay, queue saturation, memory/resource bounds, and database outage. Record actual results.

Incident 001: deterministic delayed event reproduces state regression under an isolated unsafe policy; corrected policy preserves highest accepted sequence. Demonstrate failing and passing assertions against the same fixture.

Incident 002: temporarily interrupt broker/database connectivity, replay events, and verify documented delivery outcomes, duplicate effects, resource bounds, and recovery. Differentiate broker failure from database failure.

Incident 003: noisy values around the threshold reproduce alert flooding with an isolated naive policy; hysteresis/cooldown preserve meaningful firing and clear transitions with bounded notifications.

Each incident includes problem, reproduction, captured synthetic evidence, root cause, executable acceptance criteria, and genuine corrective commit evidence. Unsafe modes are available only in an explicitly invoked incident harness, never default runtime behavior.

## Performance and retention

Measure throughput, latency distribution, duplicate/conflict counts, queue occupancy, retries, active goroutines, and memory for a declared workload and duration. Report hardware, versions, sample size, and errors; no invented target attainment. Bound metric label cardinality by excluding correlation IDs and unbounded device IDs. Logs and traces carry correlation IDs.

Initial demonstration retains events until the owner explicitly deletes the local volume. Document storage growth and lack of automatic retention; do not advertise a retention engine. Benchmark event ingestion separately from simulator overhead and database warm-up.

## Acceptance gates

1. Fresh documented setup starts the complete local stack and ingests synthetic telemetry.
2. Duplicate and delayed events preserve current device state under concurrency.
3. Failed transactions produce no partial state/alert update.
4. Dependency failures preserve documented resource bounds and processing resumes.
5. All three broken-state reproductions fail for their intended reason; fixes pass regressions.
6. Unit, race, static analysis, integration, and documented demo checks pass with saved output.
7. Logs, metrics, traces, and dashboards reflect actual running behavior.
8. Reviewer can reproduce setup and incidents without undocumented steps.
9. Security, dependency, and license review completed before public publishing approval.

## Implementation milestones

A: Go module, validated config, CI, developer commands. B: event and alert domain logic with tests. C: database migration, transactional persistence, MQTT integration. D: retries, reconnect, queue limits and shutdown. E: observability and API. F: incident harnesses and evidence. G: independent documented setup, engineering review, measured benchmark, and publication audit.

## Known limits and trade-offs

Single backend and single database; no high-availability claim. Trusted local publishers; no industrial certification. Explicit stream provisioning favors clear ordering over automatic device resets. Database persistence is durable only under the configured PostgreSQL and storage guarantees. MQTT acknowledgment behavior is an implementation decision requiring failure tests. Sixteen GB RAM requires avoiding simultaneous stacks for all three projects. Deadline pressure cannot override acceptance gates.
