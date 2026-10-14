CREATE TABLE IF NOT EXISTS devices (
 id text PRIMARY KEY,
 generation bigint NOT NULL CHECK(generation>0),
 sequence bigint NOT NULL DEFAULT 0 CHECK(sequence>=0),
 temperature_mc bigint NOT NULL DEFAULT 0,
 observed_at timestamptz,
 last_seen timestamptz,
 alert_active boolean NOT NULL DEFAULT false,
 last_notice timestamptz
);
CREATE TABLE IF NOT EXISTS telemetry (
 device text NOT NULL REFERENCES devices(id),
 generation bigint NOT NULL,
 sequence bigint NOT NULL,
 fingerprint bytea NOT NULL,
 temperature_mc bigint NOT NULL,
 observed_at timestamptz NOT NULL,
 received_at timestamptz NOT NULL,
 correlation_id text NOT NULL,
 PRIMARY KEY(device,generation,sequence)
);
CREATE TABLE IF NOT EXISTS alert_transitions (
 id bigserial PRIMARY KEY,
 device text NOT NULL REFERENCES devices(id),
 sequence bigint NOT NULL,
 state text NOT NULL CHECK(state IN ('firing','cleared')),
 notify boolean NOT NULL,
 created_at timestamptz NOT NULL,
 correlation_id text NOT NULL
);
