# Verification evidence

Local verification date: October 15, 2026, Asia/Kolkata. Evidence timestamps retain actual host UTC output. Go 1.27.2, Linux amd64, AMD Ryzen 7 7840HS, 16 logical CPUs, approximately 14 GiB usable RAM. GPU not used.

| Check | Recorded result | Evidence |
| --- | --- | --- |
| Unit tests with race detector, formatting, static analysis, build | Passed | `evidence/final-checks.txt` |
| PostgreSQL integration and wider unit suite with race detector | Passed | `evidence/integration-tests.txt` |
| Ordering defect then correction | Expected failure, corrected pass | `evidence/001-results.txt` |
| Premature acknowledgement then correction | Expected failure, corrected pass | `evidence/002-results.txt` |
| Noisy alert policy then correction | Expected failure, corrected pass | `evidence/003-results.txt` |
| Real database outage and broker reconnect | 240 deliveries → 120 unique events; observed queue maximum 14 of capacity 64 | `evidence/002-recovery.txt` |
| Go dependency vulnerability scan after remediation | No vulnerabilities found | `evidence/dependency-scan.txt` |
| Five-second fuzz request (tool ran approximately six seconds) | 393800 executions, no failures | `evidence/fuzz.txt` |
| Dashboard, metrics, and trace service API | Actual runtime responses captured | `evidence/dashboard-query.json`, `metrics-query.json`, `trace-services.json` |

## Benchmark

Three single-threaded sequential database ingestion samples, requested three seconds each: approximately 1.127582, 1.448577, and 1.104497 milliseconds per operation. Roughly 2255–2256 allocation bytes and 51 Go allocations per operation. These measure one hot device, local PostgreSQL, a bounded pool, and transaction overhead. They exclude MQTT transport, simulator work, trace export, HTTP, and end-to-end throughput. Database/container activity and host scheduling were not isolated. Do not extrapolate production capacity.

Raw Go benchmark output: `evidence/benchmark.txt`. Each benchmark created and cleaned up its own synthetic device. Warm-up and sample counts follow the Go benchmark harness; these are measurements, not service-level objectives.

## Fresh-stack and resource verification

A separate `factorymesh-review` Compose project started with fresh named volumes, then executed the documented demo and dependency recovery. Captured output: `evidence/clean-start-demo.txt` and `evidence/clean-start-recovery.txt`. During the finite outage sample, the observed peak resident memory was 29175808 bytes, maximum goroutines 43, and queued messages 14 of capacity 64. This sample is not a long-duration memory leak test.

Gitleaks 8.30.1 scanned the working files and genuine Git history with no leaks reported; see `evidence/secret-scan-files.txt` and `secret-scan-history.txt`. This is a credential-pattern check, not proof that every sensitive string is detectable. Actual coverage output is in `evidence/coverage.txt`; command entrypoints have no instrumented unit coverage, although the running stack is exercised by recovery.

## Explicit limits

Fuzzing and finite concurrency checks cannot prove all inputs or interleavings safe. Broker capacity exhaustion and hard host crash were not certified for lossless delivery; the contract explicitly permits loss at those boundaries. Local checks do not imply a remote GitHub Actions pass. Remote CI and publication status will be recorded after actual GitHub verification.
