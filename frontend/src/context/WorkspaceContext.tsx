import { createContext, useContext, useState, useEffect, type ReactNode, useMemo } from "react";
import { useDevices } from "../hooks/useArgusData";
import type { Workspace, Device } from "../types/api";
import { useAuth } from "./AuthContext";

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
  const { workspaces: authWorkspaces, isAuthenticated, activeWorkspaceId: authActiveId, setActiveWorkspaceId: authSetActiveId } = useAuth();
  
  // Fetch devices only when authenticated and an active workspace is selected
  const devicesQuery = useDevices({ enabled: isAuthenticated && Boolean(authActiveId) });

  // Map Auth context workspaces to Workspace list
  const workspaces = useMemo<Workspace[]>(() => {
    return (authWorkspaces || []).map(w => ({
      id: w.id,
      name: w.name,
      description: "",
      created_at: "",
      device_count: 0
    }));
  }, [authWorkspaces]);

  const activeWorkspace = useMemo(() => {
    return (workspaces || []).find(w => w.id === authActiveId) || null;
  }, [workspaces, authActiveId]);

  const workspaceDevices = useMemo(() => {
    if (!authActiveId || !devicesQuery.data) return [];
    return devicesQuery.data.filter(d => d.workspace_id === authActiveId);
  }, [authActiveId, devicesQuery.data]);

  return (
    <WorkspaceContext.Provider value={{
      selectedWorkspaceId: authActiveId,
      setSelectedWorkspaceId: authSetActiveId,
      workspaces,
      activeWorkspace,
      workspaceDevices,
      isLoading: devicesQuery.isLoading
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
