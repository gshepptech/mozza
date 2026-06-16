import { createContext, useContext, useState, useEffect, useCallback } from "react";
import type { ReactNode } from "react";

// ─── Types ──────────────────────────────────────────────────

interface ClusterStatus {
  reachable: boolean;
  lastSeen: string;
  error: string;
  stale: boolean;
}

interface ClusterContextValue {
  status: ClusterStatus;
  loading: boolean;
  refresh: () => void;
}

const defaultStatus: ClusterStatus = {
  reachable: false,
  lastSeen: "",
  error: "",
  stale: false,
};

// ─── Context ────────────────────────────────────────────────

const ClusterCtx = createContext<ClusterContextValue>({
  status: defaultStatus,
  loading: true,
  refresh: () => {},
});

// ─── Provider ───────────────────────────────────────────────

export function ClusterStatusProvider({ children }: { children: ReactNode }) {
  const [status, setStatus] = useState<ClusterStatus>(defaultStatus);
  const [loading, setLoading] = useState(true);

  const fetchStatus = useCallback(async () => {
    try {
      const resp = await fetch("/api/v1/cluster/status");
      if (resp.ok) {
        const data = await resp.json();
        setStatus({
          reachable: data.reachable ?? false,
          lastSeen: data.last_seen ?? "",
          error: data.error ?? "",
          stale: data.stale ?? false,
        });
      } else {
        setStatus({
          reachable: false,
          lastSeen: "",
          error: "Status endpoint unavailable",
          stale: false,
        });
      }
    } catch {
      setStatus({
        reachable: false,
        lastSeen: "",
        error: "Cannot reach server",
        stale: false,
      });
    } finally {
      setLoading(false);
    }
  }, []);

  // Fetch on mount + poll every 15s
  useEffect(() => {
    fetchStatus();
    const interval = setInterval(fetchStatus, 15_000);
    return () => clearInterval(interval);
  }, [fetchStatus]);

  return (
    <ClusterCtx.Provider value={{ status, loading, refresh: fetchStatus }}>
      {children}
    </ClusterCtx.Provider>
  );
}

// ─── Hook ───────────────────────────────────────────────────

export function useClusterStatus(): ClusterContextValue {
  return useContext(ClusterCtx);
}
