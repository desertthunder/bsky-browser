-- Add session_id column to auth table for OAuth session management
ALTER TABLE auth ADD COLUMN session_id TEXT;
