# Security scope

The stack is a trusted, local, synthetic-data demonstration. Ports bind to loopback, the Go container runs as an unprivileged user, SQL uses bound parameters, payload size is bounded, and HTTP has header/read/write/idle timeouts. CI permissions are read-only for repository contents. No personal, employer, or customer data is used.

The MQTT broker deliberately permits anonymous local publishing. This is not a production authentication model. PostgreSQL's tracked password is an explicit public demonstration value. Use real credentials through environment variables in a separate deployment; never commit them. Current PostgreSQL demonstration user owns its schema because it initializes tables; a production rollout needs separated migration and runtime roles.

Go dependencies are checked by `govulncheck`; captured results are in `evidence/dependency-scan.txt`. The initial scan and remediation are preserved separately. A passing scan does not prove absence of security defects. Container image vulnerabilities and hostile multi-tenant traffic require additional review before any production deployment.

No remote/cloud exposure, TLS termination, device certificates, authorization system, durable rejection quarantine, or tenant isolation is implemented. Do not deploy this Compose file to a publicly reachable machine unchanged.
