-- Recreate tables dropped in migration 000029
CREATE TABLE IF NOT EXISTS session_events (
    id UUID PRIMARY KEY,
    session_id UUID NOT NULL,
    device_id VARCHAR(255) NOT NULL,
    event_type VARCHAR(255) NOT NULL,
    severity VARCHAR(50) NOT NULL,
    payload JSONB NOT NULL,
    created_at TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS session_alerts (
    id UUID PRIMARY KEY,
    session_id UUID NOT NULL,
    device_id VARCHAR(255) NOT NULL,
    severity VARCHAR(50) NOT NULL,
    message TEXT NOT NULL,
    resolved BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL,
    resolved_at TIMESTAMP
);

CREATE TABLE IF NOT EXISTS session_reports (
    id UUID PRIMARY KEY,
    session_id UUID NOT NULL,
    report_json JSONB NOT NULL,
    generated_at TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS session_ai_reports (
    id UUID PRIMARY KEY,
    session_id UUID NOT NULL,
    summary_text TEXT NOT NULL,
    metadata JSONB,
    generated_at TIMESTAMP NOT NULL
);
