-- 000001_init.down.sql
-- Rollback initial schema

DROP TABLE IF EXISTS refresh_tokens;
DROP TABLE IF EXISTS clips;
DROP TABLE IF EXISTS slot_participations;
DROP TABLE IF EXISTS session_participants;
DROP TABLE IF EXISTS session_slots;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS users;

DROP TYPE IF EXISTS slot_participation_status;
DROP TYPE IF EXISTS participant_status;
DROP TYPE IF EXISTS session_status;
DROP TYPE IF EXISTS session_mode;

DROP EXTENSION IF EXISTS pgcrypto;
