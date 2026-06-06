-- Migration to drop the NOT NULL constraint on users.password_hash
ALTER TABLE users ALTER COLUMN password_hash DROP NOT NULL;
