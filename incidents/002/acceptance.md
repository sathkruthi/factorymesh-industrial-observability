# Executable acceptance

No acknowledgement before persistence succeeds. Queue admission remains bounded and cancellation terminates blocked work. Within configured broker capacity, 240 deliveries recover to 120 unique stored events, and broker reconnect preserves state.

Run `make incident ID=002`, then `make test check build integration`. The focal regression is `TestAckOnlyAfterCommitAndRecovery`; wider integration checks cover transaction rollback, concurrent permutations, duplicate conflicts, and late-event alert preservation.

Corrective implementation milestone: genuine commit `926dd92` (`feat: implement transactional telemetry and bounded ingestion`). Later regression/evidence commits extend verification. No backdated or fabricated commits.
