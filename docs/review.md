# Engineering review

Scope: original FactoryMesh implementation, local Compose deployment, tests, incident contracts, dependency scan, and documentation. This is an author-conducted review, not independent third-party approval.

## Findings addressed

| Severity | Finding | Resolution and evidence |
| --- | --- | --- |
| High | Indirect HTTP dependency scan reported five vulnerabilities | Updated `golang.org/x/net` from 0.59.0 to 0.61.0; final govulncheck reports no vulnerabilities; original scan retained |
| High | Receipt-time acknowledgement can lose uncommitted work | Manual acknowledgements follow successful transaction commit; incident 002 fails for premature ack and passes corrected handling |
| High | Last-arrival-wins state allows delayed-event corruption | Device row locking and sequence guard; incident 001 and concurrent permutation tests |
| Medium | Missing/null numeric JSON fields could silently become zero | Explicit required/non-null field checks plus regression tests; zero Celsius remains valid when explicitly provided |
| Medium | Alert transitions could partially persist on an error | Single transaction; injected alert constraint error proves event/state rollback |
| Medium | Threshold oscillation can flood eligible notifications | Hysteresis and persistent cooldown; incident 003 retains meaningful firing |
| Medium | Fresh demo replay could conflict with existing sequences | Demo command reads current state and chooses the next sequence; parallel demos still unsupported |
| Low | Grafana default port conflicted with an existing application | Project uses loopback port 3300; unrelated application retained |
| Low | Recovery unit test could race before its first injected failure | Wait for confirmed failed storage attempt before enabling recovery |

## Review conclusions

Architecture: one ingestion service keeps durable boundaries understandable; per-device serialization is intentional. Different devices can process concurrently. Single replica and single database remain availability limits.

Correctness: tests cover duplicates, conflicting replay, unregistered streams, concurrent sequence permutations, stale alert suppression, and transaction rollback. Event-time and receipt-time contracts are explicit. Tests do not establish correctness for every possible runtime or failure interleaving.

Reliability: application admission and worker counts are bounded; broker queue limits are finite; SQL retries have deadlines and capped jittered backoff. Retrying indefinitely keeps uncommitted messages unacknowledged but requires operator repair for permanent SQL faults. Broker hard-crash loss and queue overflow are documented, not hidden behind an exactly-once claim.

Security: synthetic data only; loopback port mappings; unprivileged app container; parameterized SQL; constrained payloads/timeouts; read-only CI permissions; credential-pattern review. MQTT anonymous access and schema-owner database privileges are acceptable only inside the explicitly trusted local demo boundary. Container CVE scanning and production authentication remain additional deployment work.

Testing: real PostgreSQL tests isolate schemas; live recovery exercises actual dependency interruption; unsafe test policies fail for the intended assertion before correction. Fuzzing checks malformed-input safety and replay identity preservation. A workload result is not universal concurrency or performance proof.

Documentation: README commands correspond to developer targets; incident materials link real test fixtures and captured output. State limitations and notification eligibility are described precisely. The initial design's shutdown-drain and generation-rotation ideas were reduced to explicitly documented cancellation/redelivery and fixed registered generations.

## Residual limitations

- No industrial authentication, multi-tenant authorization, HA, automatic stream rotation, or retention engine.
- No external notification transport or durable rejected-event quarantine.
- One fixed MQTT subscriber ID prevents horizontal replicas.
- Broker persistence is periodic; no losslessness claim across hard broker/host crashes.
- Queue bounds were exercised under a finite workload; no long-duration stress certification.
- Container image vulnerability scanning remains outside the recorded Go dependency scan.
- No production deployment, real users, independent reviewer, or employment claims.

Original code remains unlicensed by owner choice. Third-party compiled dependency license texts are preserved in [notices](third-party-notices.md).
