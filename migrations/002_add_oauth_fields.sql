-- Migration 002: Add OAuth session fields for proper token refresh support

ALTER TABLE auth ADD COLUMN auth_server_url TEXT;
ALTER TABLE auth ADD COLUMN dpop_auth_nonce TEXT;
ALTER TABLE auth ADD COLUMN dpop_host_nonce TEXT;
ALTER TABLE auth ADD COLUMN dpop_private_key TEXT;
