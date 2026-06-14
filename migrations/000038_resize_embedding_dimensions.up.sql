ALTER TABLE events ALTER COLUMN embedding TYPE vector(384) USING NULL;
ALTER TABLE operational_memory ALTER COLUMN embedding TYPE vector(384) USING NULL;
