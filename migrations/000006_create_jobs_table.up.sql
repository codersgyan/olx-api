CREATE TABLE jobs (
    id BIGSERIAL PRIMARY KEY,
    kind TEXT NOT NULL,
    payload JSONB NOT NULL,
    attempts SMALLINT NOT NULL DEFAULT 0,
    last_error TEXT,
    run_after TIMESTAMPTZ NOT NULL DEFAULT NOW (),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW ()
);

CREATE INDEX idx_jobs_claim ON jobs (run_after);