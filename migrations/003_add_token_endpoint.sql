-- Migration 003: Add auth server token endpoint for OAuth refresh

ALTER TABLE auth ADD COLUMN auth_server_token_endpoint TEXT;
ALTER TABLE auth ADD COLUMN auth_server_revocation_endpoint TEXT;
