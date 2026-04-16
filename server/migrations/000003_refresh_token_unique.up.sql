-- A UNIQUE constraint on token_hash guarantees GetByHash can never return an
-- ambiguous row. The plain index from 000001 is redundant — UNIQUE creates its
-- own supporting index.
DROP INDEX IF EXISTS idx_refresh_tokens_hash;

ALTER TABLE refresh_tokens
    ADD CONSTRAINT refresh_tokens_token_hash_key UNIQUE (token_hash);
