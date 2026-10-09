-- Rules were global; scope them to a workspace so evaluation cannot raise
-- alerts across tenants. Pre-existing rows keep NULL and are not evaluated.
ALTER TABLE rules ADD COLUMN IF NOT EXISTS workspace_id UUID;

CREATE INDEX IF NOT EXISTS idx_rules_workspace_id ON rules(workspace_id);
