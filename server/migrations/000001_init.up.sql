-- 000001_init.up.sql
-- Initial database schema for Velo

-- Enable pgcrypto for gen_random_uuid()
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- ============================================================================
-- ENUM types
-- ============================================================================

CREATE TYPE session_mode AS ENUM ('named_slots', 'auto_slot');
CREATE TYPE session_status AS ENUM ('active', 'generating', 'complete', 'failed', 'cancelled');
CREATE TYPE participant_status AS ENUM ('active', 'excluded');
CREATE TYPE slot_participation_status AS ENUM ('recording', 'skipped');

-- ============================================================================
-- Tables
-- ============================================================================

CREATE TABLE users (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    apple_sub    TEXT        NOT NULL UNIQUE,
    display_name TEXT,
    avatar_url   TEXT,
    apns_token   TEXT,
    created_at   TIMESTAMPTZ NOT NULL    DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL    DEFAULT now()
);

CREATE TABLE sessions (
    id                     UUID           PRIMARY KEY DEFAULT gen_random_uuid(),
    creator_id             UUID           REFERENCES users (id) ON DELETE SET NULL,
    name                   TEXT,
    mode                   session_mode   NOT NULL,
    section_count          INT            NOT NULL CHECK (section_count BETWEEN 1 AND 6),
    max_section_duration_s INT            NOT NULL CHECK (max_section_duration_s IN (10, 15, 20, 30)),
    deadline               TIMESTAMPTZ    NOT NULL,
    invite_token           TEXT           NOT NULL UNIQUE,
    status                 session_status NOT NULL    DEFAULT 'active',
    reel_url               TEXT,
    retry_count            INT            NOT NULL    DEFAULT 0,
    reminder_2h_sent       BOOLEAN        NOT NULL    DEFAULT false,
    reminder_30m_sent      BOOLEAN        NOT NULL    DEFAULT false,
    created_at             TIMESTAMPTZ    NOT NULL    DEFAULT now(),
    updated_at             TIMESTAMPTZ    NOT NULL    DEFAULT now(),
    completed_at           TIMESTAMPTZ
);

CREATE TABLE session_slots (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id UUID NOT NULL REFERENCES sessions (id) ON DELETE CASCADE,
    name       TEXT NOT NULL,
    starts_at  TIME NOT NULL,
    ends_at    TIME NOT NULL,
    slot_order INT  NOT NULL
);

CREATE TABLE session_participants (
    id                    UUID               PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id            UUID               NOT NULL REFERENCES sessions (id) ON DELETE CASCADE,
    user_id               UUID               REFERENCES users (id) ON DELETE SET NULL,
    display_name_snapshot TEXT               NOT NULL,
    joined_at             TIMESTAMPTZ        NOT NULL    DEFAULT now(),
    status                participant_status NOT NULL    DEFAULT 'active'
);

CREATE TABLE slot_participations (
    id      UUID                     PRIMARY KEY DEFAULT gen_random_uuid(),
    slot_id UUID                     NOT NULL REFERENCES session_slots (id) ON DELETE CASCADE,
    user_id UUID                     NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    status  slot_participation_status NOT NULL   DEFAULT 'recording'
);

CREATE TABLE clips (
    id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id          UUID        NOT NULL REFERENCES sessions (id) ON DELETE CASCADE,
    user_id             UUID        REFERENCES users (id) ON DELETE SET NULL,
    slot_id             UUID        REFERENCES session_slots (id) ON DELETE SET NULL,
    s3_key              TEXT        NOT NULL UNIQUE,
    recorded_at         TIMESTAMPTZ NOT NULL,
    arrived_at          TIMESTAMPTZ NOT NULL,
    recorded_at_clamped BOOLEAN     NOT NULL    DEFAULT false,
    duration_ms         INT         NOT NULL    CHECK (duration_ms > 0),
    created_at          TIMESTAMPTZ NOT NULL    DEFAULT now()
);

CREATE TABLE refresh_tokens (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash TEXT        NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL    DEFAULT now()
);

-- ============================================================================
-- Indexes
-- ============================================================================

-- Scheduler: find active sessions past their deadline
CREATE INDEX idx_sessions_deadline_status
    ON sessions (deadline)
    WHERE status = 'active';

-- Join flow: check "1 active session per user" constraint
CREATE INDEX idx_participants_user_active
    ON session_participants (user_id)
    WHERE status = 'active';

-- Prevent duplicate joins (NULLs from account deletion are excluded)
CREATE UNIQUE INDEX idx_participants_session_user
    ON session_participants (session_id, user_id)
    WHERE user_id IS NOT NULL;

-- One skip/recording decision per user per slot
CREATE UNIQUE INDEX idx_slot_participations_slot_user
    ON slot_participations (slot_id, user_id);

-- Reel generation: fetch all clips for a session
CREATE INDEX idx_clips_session
    ON clips (session_id);

-- Token refresh: look up by hash
CREATE INDEX idx_refresh_tokens_hash
    ON refresh_tokens (token_hash);

-- Fetch slots for a session
CREATE INDEX idx_slots_session
    ON session_slots (session_id);
