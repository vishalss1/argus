import { createContext, useContext, useState, useEffect, type ReactNode, useTransition } from "react";
import { request } from "../services/http";

export interface User {
  id: string;
  email: string;
  name: string;
  status: string;
  created_at: string;
}

export interface WorkspaceInfo {
  id: string;
  name: string;
}

interface AuthContextType {
  user: User | null;
  workspaces: WorkspaceInfo[];
  activeWorkspaceId: string;
  setActiveWorkspaceId: (id: string) => void;
  isAuthenticated: boolean;
  isLoading: boolean;
  login: (accessToken: string) => Promise<void>;
  logout: () => Promise<void>;
  refreshMe: () => Promise<void>;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [workspaces, setWorkspaces] = useState<WorkspaceInfo[]>([]);
  const [activeWorkspaceId, setActiveWorkspaceIdState] = useState<string>(() => {
    return localStorage.getItem("argus_active_workspace_id") || "";
  });
  const [isAuthenticated, setIsAuthenticated] = useState<boolean>(false);
  const [isLoading, setIsLoading] = useState<boolean>(true);
  const [, startTransition] = useTransition();

  const setActiveWorkspaceId = (id: string) => {
    setActiveWorkspaceIdState(id);
    if (id) {
      localStorage.setItem("argus_active_workspace_id", id);
    } else {
      localStorage.removeItem("argus_active_workspace_id");
    }
  };

  const refreshMe = async () => {
    const accessToken = localStorage.getItem("argus_access_token");
    if (!accessToken) {
      setUser(null);
      setWorkspaces([]);
      setIsAuthenticated(false);
      setIsLoading(false);
      return;
    }

    try {
      const data = await request<{ user: User; workspaces: WorkspaceInfo[] }>("/auth/me");
      setUser(data.user);
      const workspacesList = data.workspaces || [];
      setWorkspaces(workspacesList);
      setIsAuthenticated(true);

      // Automatically select workspace if not set or invalid
      if (workspacesList.length > 0) {
        const exists = workspacesList.some(w => w.id === activeWorkspaceId);
        if (!activeWorkspaceId || !exists) {
          setActiveWorkspaceId(workspacesList[0].id);
        }
      } else {
        setActiveWorkspaceId("");
      }
    } catch (err) {
      setUser(null);
      setWorkspaces([]);
      setIsAuthenticated(false);
      localStorage.removeItem("argus_access_token");
      localStorage.removeItem("argus_active_workspace_id");
    } finally {
      setIsLoading(false);
    }
  };

  const login = async (accessToken: string) => {
    localStorage.setItem("argus_access_token", accessToken);
    setIsLoading(true);
    await refreshMe();
  };

  const logout = async () => {
    try {
      await request("/auth/logout", {
        method: "POST",
      });
    } catch {
      // Ignore logout errors
    } finally {
      localStorage.removeItem("argus_access_token");
      localStorage.removeItem("argus_active_workspace_id");
      startTransition(() => {
        setUser(null);
        setWorkspaces([]);
        setActiveWorkspaceId("");
        setIsAuthenticated(false);
      });
    }
  };

  // Run startup authentication check
  useEffect(() => {
    refreshMe();
  }, []);

  return (
    <AuthContext.Provider
      value={{
        user,
        workspaces,
        activeWorkspaceId,
        setActiveWorkspaceId,
        isAuthenticated,
        isLoading,
        login,
        logout,
        refreshMe,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  const context = useContext(AuthContext);
  if (context === undefined) {
    throw new Error("useAuth must be used within an AuthProvider");
  }
  return context;
}
