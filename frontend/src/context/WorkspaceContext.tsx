import { createContext, useContext, useState, useEffect, type ReactNode, useMemo } from "react";
import { useWorkspaces, useDevices } from "../hooks/useArgusData";
import type { Workspace, Device } from "../types/api";

interface WorkspaceContextType {
  selectedWorkspaceId: string;
  setSelectedWorkspaceId: (id: string) => void;
  workspaces: Workspace[];
  activeWorkspace: Workspace | null;
  workspaceDevices: Device[];
  isLoading: boolean;
}

const WorkspaceContext = createContext<WorkspaceContextType | undefined>(undefined);

export function WorkspaceProvider({ children }: { children: ReactNode }) {
  const workspacesQuery = useWorkspaces();
  const devicesQuery = useDevices();

  const [selectedWorkspaceId, setSelectedWorkspaceId] = useState<string>(() => {
    return localStorage.getItem("argus_active_workspace_id") || "";
  });

  const workspaces = workspacesQuery.data ?? [];

  // Automatically select first workspace if none is selected or selected one no longer exists
  useEffect(() => {
    if (workspaces.length > 0) {
      const exists = workspaces.some(w => w.id === selectedWorkspaceId);
      if (!selectedWorkspaceId || !exists) {
        const firstId = workspaces[0].id;
        setSelectedWorkspaceId(firstId);
        localStorage.setItem("argus_active_workspace_id", firstId);
      }
    } else if (selectedWorkspaceId) {
      // Clear if empty
      setSelectedWorkspaceId("");
      localStorage.removeItem("argus_active_workspace_id");
    }
  }, [workspaces, selectedWorkspaceId]);

  const activeWorkspace = useMemo(() => {
    return workspaces.find(w => w.id === selectedWorkspaceId) || null;
  }, [workspaces, selectedWorkspaceId]);

  const workspaceDevices = useMemo(() => {
    if (!selectedWorkspaceId || !devicesQuery.data) return [];
    return devicesQuery.data.filter(d => d.workspace_id === selectedWorkspaceId);
  }, [selectedWorkspaceId, devicesQuery.data]);

  const handleSetWorkspaceId = (id: string) => {
    setSelectedWorkspaceId(id);
    localStorage.setItem("argus_active_workspace_id", id);
  };

  return (
    <WorkspaceContext.Provider value={{
      selectedWorkspaceId,
      setSelectedWorkspaceId: handleSetWorkspaceId,
      workspaces,
      activeWorkspace,
      workspaceDevices,
      isLoading: workspacesQuery.isLoading || devicesQuery.isLoading
    }}>
      {children}
    </WorkspaceContext.Provider>
  );
}

export function useWorkspaceContext() {
  const context = useContext(WorkspaceContext);
  if (context === undefined) {
    throw new Error("useWorkspaceContext must be used within a WorkspaceProvider");
  }
  return context;
}
