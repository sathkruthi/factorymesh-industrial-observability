# Root cause

The unsafe rule sets clear equal to high and cooldown to zero. Correct rules use 75000 clear versus 80000 high, persist state transitions, and suppress new firing notifications during a one-minute cooldown. A later genuine firing remains eligible.

Trace the behavior through `internal/store/store_integration_test.go` and its production implementation. This is a reproducible controlled reenactment, not fabricated customer history.
