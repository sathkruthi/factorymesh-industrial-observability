# Root cause

The unsafe test policy bypasses the sequence guard. Correct processing locks the device row, records every new valid event, and advances current state only for a strictly higher sequence in the registered generation.

Trace the behavior through `internal/store/store_integration_test.go` and its production implementation. This is a reproducible controlled reenactment, not fabricated customer history.
