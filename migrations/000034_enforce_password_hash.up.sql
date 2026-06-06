-- Migration to enforce NOT NULL constraint on users.password_hash after removing Google OAuth

-- Alter the column to be NOT NULL
ALTER TABLE users ALTER COLUMN password_hash SET NOT NULL;
