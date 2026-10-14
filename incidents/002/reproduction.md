# Reproduction

Start dependencies with `make up`, then run `make incident ID=002`. The harness requires the unsafe policy to fail for its expected reason, then runs the same regression with the corrected policy.

Read `../../evidence/002-results.txt` for captured execution. Fresh results are written to `evidence-local/`. Test-only fault flags are removed between runs.

Run `make recovery` for real database interruption and broker restart. Dependencies are restored in a finally block.
