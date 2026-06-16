import { useState, useEffect, useRef, useCallback } from "react";

// ─── Types ──────────────────────────────────────────────────

interface ClusterDataState<T> {
  data: T | null;
  loading: boolean;
  error: string | null;
  errorCode: string | null;
  stale: boolean;
  refresh: () => void;
}

interface CacheEntry<T> {
  data: T;
  timestamp: number;
  stale: boolean;
  errorCode: string | null;
}

// ─── Global cache (shared across hook instances) ────────────

const cache = new Map<string, CacheEntry<unknown>>();
const inflight = new Map<string, Promise<unknown>>();

const DEFAULT_TTL_MS = 15_000; // 15 seconds

// ─── Hook ───────────────────────────────────────────────────

/**
 * Fetches cluster data with caching, deduplication, and stale-while-revalidate.
 *
 * - Multiple components calling useClusterData("pods") share a single fetch.
 * - Cached responses served instantly; background refresh after TTL.
 * - Manual refresh() bypasses cache.
 */
export function useClusterData<T>(
  key: string,
  fetcher: () => Promise<T>,
  ttlMs: number = DEFAULT_TTL_MS,
): ClusterDataState<T> {
  const [data, setData] = useState<T | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [errorCode, setErrorCode] = useState<string | null>(null);
  const [stale, setStale] = useState(false);
  const mountedRef = useRef(true);

  const doFetch = useCallback(
    async (bypassCache: boolean) => {
      // Check cache
      if (!bypassCache) {
        const cached = cache.get(key) as CacheEntry<T> | undefined;
        if (cached && Date.now() - cached.timestamp < ttlMs) {
          setData(cached.data);
          setStale(cached.stale);
          setLoading(false);
          setError(null);
          setErrorCode(cached.errorCode);
          return;
        }
        // Stale-while-revalidate: show stale data while refreshing
        if (cached) {
          setData(cached.data);
          setStale(true);
          setLoading(false);
        }
      }

      // Deduplicate: if a fetch is already in flight for this key, wait for it
      let promise = inflight.get(key) as Promise<T> | undefined;
      if (!promise) {
        promise = fetcher();
        inflight.set(key, promise);
      }

      try {
        const result = await promise;
        if (!mountedRef.current) return;

        cache.set(key, {
          data: result,
          timestamp: Date.now(),
          stale: false,
          errorCode: null,
        });
        setData(result);
        setStale(false);
        setError(null);
        setErrorCode(null);
        setLoading(false);
      } catch (err: unknown) {
        if (!mountedRef.current) return;

        // Try to extract error code from API response
        let code: string | null = null;
        let msg = "Failed to fetch";
        if (err && typeof err === "object") {
          const e = err as Record<string, unknown>;
          if (typeof e.code === "string") code = e.code;
          if (typeof e.message === "string") msg = e.message;
          else if (typeof e.error === "string") msg = e.error;
        }
        setError(msg);
        setErrorCode(code);
        setLoading(false);
      } finally {
        inflight.delete(key);
      }
    },
    [key, fetcher, ttlMs],
  );

  useEffect(() => {
    mountedRef.current = true;
    doFetch(false);
    return () => {
      mountedRef.current = false;
    };
  }, [doFetch]);

  const refresh = useCallback(() => {
    cache.delete(key);
    setLoading(true);
    doFetch(true);
  }, [key, doFetch]);

  return { data, loading, error, errorCode, stale, refresh };
}

/**
 * Invalidate all cached cluster data (e.g., on cluster/team switch).
 */
export function invalidateClusterCache(): void {
  cache.clear();
}
