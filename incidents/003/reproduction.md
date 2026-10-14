# Reproduction

Start dependencies with `make up`, then run `make incident ID=003`. The harness requires the unsafe policy to fail for its expected reason, then runs the same regression with the corrected policy.

Read `../../evidence/003-results.txt` for captured execution. Fresh results are written to `evidence-local/`. Test-only fault flags are removed between runs.
