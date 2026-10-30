-- Failed-login counters for rate limiting. Kept in Postgres rather than in
-- memory so every serverless instance sees the same counts. Keys are opaque
-- ("ip:<addr>" or "email:<sha256>"), so no plaintext emails are stored here.
CREATE TABLE IF NOT EXISTS login_failures (
    key          TEXT        PRIMARY KEY,
    failures     INT         NOT NULL,
    window_start TIMESTAMPTZ NOT NULL
);

-- Lets the opportunistic cleanup in RecordFailure find expired rows cheaply.
CREATE INDEX IF NOT EXISTS login_failures_window_idx ON login_failures (window_start);

-- Same reasoning as 002: hide the table from Supabase's public API.
ALTER TABLE login_failures ENABLE ROW LEVEL SECURITY;
