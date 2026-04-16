ALTER TABLE refresh_tokens DROP CONSTRAINT IF EXISTS refresh_tokens_token_hash_key;

CREATE INDEX idx_refresh_tokens_hash ON refresh_tokens (token_hash);
