# Executable acceptance

Highest accepted sequence is 3 and its temperature is 70003. All valid late records persist. Conflicting duplicates and unregistered generations cannot mutate current state.

Run `make incident ID=001`, then `make test check build integration`. The focal regression is `TestIncident001Ordering`; wider integration checks cover transaction rollback, concurrent permutations, duplicate conflicts, and late-event alert preservation.

Corrective implementation milestone: genuine commit `926dd92` (`feat: implement transactional telemetry and bounded ingestion`). Later regression/evidence commits extend verification. No backdated or fabricated commits.
