# Root cause

The unsafe test harness emulates receipt-time acknowledgement and fails the commit-before-ack assertion. Correct workers retain unacknowledged messages during bounded retries. The live Compose exercise adds database interruption, repeated delivery, broker interruption, and reconnect.

Trace the behavior through `internal/pipeline/pipeline_test.go` and its production implementation. This is a reproducible controlled reenactment, not fabricated customer history.
