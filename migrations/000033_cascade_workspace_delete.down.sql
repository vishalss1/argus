-- Migration to restore workspace_id foreign key constraint on devices to ON DELETE SET NULL
ALTER TABLE devices DROP CONSTRAINT IF EXISTS devices_workspace_id_fkey;

ALTER TABLE devices 
    ADD CONSTRAINT devices_workspace_id_fkey 
    FOREIGN KEY (workspace_id) 
    REFERENCES workspaces(id) 
    ON DELETE SET NULL;
