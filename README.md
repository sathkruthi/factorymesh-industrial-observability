# FactoryMesh

**Industrial telemetry reliability and incident investigation, with executable failure reproductions.**

FactoryMesh receives synthetic MQTT equipment readings, commits them to PostgreSQL, protects current device state against delayed and duplicate messages, and records alert transitions. This is a local production-style engineering exercise, not a deployed industrial product.

## Review in five minutes

1. Start the stack: `make up`.
2. Generate telemetry: `make demo`.
3. Run checks: `make test check build integration`.
4. Reproduce the ordering defect and correction: `make incident ID=001`.
5. Read [root cause](incidents/001/root-cause.md), [acceptance criteria](incidents/001/acceptance.md), and [captured failing/passing results](evidence/001-results.txt).

The incident command succeeds only if the unsafe policy fails for its intended reason and the corrected policy passes. Unsafe policies exist only in tests; they cannot be enabled in the running service.

## Architecture

```mermaid
flowchart LR
    S[Synthetic device simulator] -->|MQTT QoS 1| B[Mosquitto persistent session]
    B -->|manual acknowledgement| Q[Bounded admission queue]
    Q --> W[Go ingestion workers]
    W -->|one transaction| D[(PostgreSQL events, state, alerts)]
    W --> O[Structured logs and OpenTelemetry]
    D --> A[Read-only HTTP APIs]
    W --> M[Prometheus metrics]
    M --> G[Grafana dashboard]
    O --> J[Jaeger traces]
```

One Go service owns validation, state ordering, and alert transitions. PostgreSQL serializes updates per device. No external notification delivery is implemented: `notify=true` records notification eligibility for inspection.

## Local setup

Prerequisites: Linux, Docker with Compose, Go 1.27.2 or newer, Python 3, Make, and available local ports 1883, 55432, 8080, 9090, 3300, and 16686. Docker-only startup needs no Go installation; tests and incident harnesses require Go. All published ports bind to loopback. The tested machine has approximately 14 GiB usable RAM; container memory limits total less than 2 GiB excluding build overhead.

```sh
make setup
make up
make demo
curl http://127.0.0.1:8080/devices
curl http://127.0.0.1:8080/alerts
```

`make demo` obtains the next sequence from the current device state, so sequential reruns work. Do not run two demos for the same device concurrently: they would reserve overlapping sequences. Demo fixtures are synthetic; temperature is integer milli-degrees Celsius.

- API: http://127.0.0.1:8080/devices
- Dashboard: http://127.0.0.1:3300/d/factorymesh
- Metrics: http://127.0.0.1:9090
- Trace explorer: http://127.0.0.1:16686

`make down` stops this stack while retaining its database and broker volumes. It does not delete data. The sample password `local_demo_only` is public development configuration, never a production credential.

Three devices are provisioned at startup: `press-001` through `press-003`, generation 1. Unknown devices/generations are rejected. General device management and stream rotation are outside this release; see [contracts](docs/contracts.md).

## Verification

```sh
make test check build
make integration
make incident ID=001
make incident ID=002
make incident ID=003
make recovery
```

`make recovery` temporarily stops only this Compose project's database and broker, publishes duplicates during the database outage, checks readiness/liveness and queue bounds, then restores dependencies and verifies persisted results. It requires a local Unix-socket Docker context. Use a disposable local demonstration stack, never a real equipment gateway.

Tests isolate database tables in unique schemas; they do not erase demonstration data. Integration tests fail when dependencies are missing, rather than silently skipping. The benchmark creates and cleans up its own synthetic device:

```sh
DATABASE_URL='postgres://factorymesh:local_demo_only@localhost:55432/factorymesh?sslmode=disable' go test -tags=integration -run '^$' -bench BenchmarkIngest -benchtime=3s -count=3 ./internal/store
```

Measured evidence is stored in [evidence](evidence/). [Verification report](docs/verification.md) explains what was actually exercised, workload limits, and known gaps. CI configuration runs unit/race/static checks, a Go dependency scan, the Compose stack, integration tests, all three incident checks, and recovery. A workflow file is not proof of a remote CI pass; publication status is recorded separately.

## Guarantees and limits

- Identical `(device, generation, sequence)` replay has one database effect. Conflicting replay is rejected.
- Current state advances only for higher sequences within the provisioned generation. Late readings remain available as investigation evidence.
- Event insertion, current state, and alert transitions commit together or roll back together.
- MQTT acknowledgements occur after database commit, or after explicit permanent payload rejection. A commit with an uncertain acknowledgement can replay safely.
- Delivery is not exactly once. Publisher acknowledgement means broker receipt, not database persistence. Finite broker queues, broker persistence intervals, and a hard broker/host crash can lose messages. The outage test covers traffic within those limits.
- Application queue capacity is configurable and bounded; worker count, connection pool, payload sizes, and container memory are also bounded. SQL failures retry with capped exponential backoff and jitter until cancellation.
- Alert hysteresis prevents threshold noise from repeatedly clearing/firing. Cooldown suppresses notification eligibility while preserving state transitions. Only current-state advances affect alerts.
- Shutdown cancels workers; uncommitted messages remain unacknowledged for broker redelivery. This release does not promise a full shutdown drain.

Read the [design](docs/design.md), [contracts](docs/contracts.md), [operations](docs/runbook.md), [decisions](docs/decisions.md), [security notes](docs/security.md), and [engineering review](docs/review.md).

## Incident index

| Incident | Failure | Correctness evidence |
| --- | --- | --- |
| [001](incidents/001/problem.md) | Delayed telemetry overwrites newer state | Same fixture produces sequence 2 under unsafe policy and 3 under corrected policy |
| [002](incidents/002/problem.md) | Gateway acknowledges before durable persistence | Commit-before-ack regression plus real dependency outage and broker reconnect |
| [003](incidents/003/problem.md) | Threshold noise floods alert notifications | Same fixture produces 5 unsafe notifications versus 2 eligible corrected notifications |

These are explicitly synthetic engineering exercises, not claims about historical customer incidents or professional employment.

## Running-system evidence

![Actual Grafana dashboard during a synthetic telemetry run](evidence/dashboard.png)

Captured from the running local stack after publishing 300 synthetic readings. Charts show actual Prometheus observations; no dashboard values were drawn or invented. Processing latency includes worker transaction time, and this short run does not establish sustained throughput.

## Ownership and licensing

Original project code is unlicensed at the owner’s request. Third-party compiled dependency license texts are preserved in [third-party notices](docs/third-party-notices.md).
