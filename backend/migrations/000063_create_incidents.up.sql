-- Incident tracking for the status page.
-- Admin creates/updates incidents via API; frontend displays them automatically.

CREATE TABLE incidents (
    id                VARCHAR(20)  PRIMARY KEY,  -- 'INC-2026-0227'
    title             VARCHAR(255) NOT NULL,
    status            VARCHAR(20)  NOT NULL DEFAULT 'investigating',
    severity          VARCHAR(10)  NOT NULL DEFAULT 'minor',
    affected_services TEXT[],                     -- e.g. {'api','database'}
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    resolved_at       TIMESTAMPTZ
);

CREATE TABLE incident_updates (
    id          BIGSERIAL    PRIMARY KEY,
    incident_id VARCHAR(20)  NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    status      VARCHAR(20)  NOT NULL,
    message     TEXT         NOT NULL,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_incidents_created_at ON incidents (created_at DESC);
CREATE INDEX idx_incident_updates_incident ON incident_updates (incident_id, created_at DESC);
