# Executable acceptance

Unsafe fixture produces 5 eligible notifications and 9 transitions. Corrected fixture produces 2 eligible notifications and 5 transitions; final state remains firing. Late readings cannot clear current alerts.

Run `make incident ID=003`, then `make test check build integration`. The focal regression is `TestIncident003AlertNoise`; wider integration checks cover transaction rollback, concurrent permutations, duplicate conflicts, and late-event alert preservation.

Corrective implementation milestone: genuine commit `926dd92` (`feat: implement transactional telemetry and bounded ingestion`). Later regression/evidence commits extend verification. No backdated or fabricated commits.
