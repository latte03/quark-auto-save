import { useCallback, useState, useSyncExternalStore } from "react";
import { browse, type BrowseEntry } from "../../lib/api";

export type DirectoryState = {
  entries: BrowseEntry[];
  loading: boolean;
  error: string;
  updatedAt: number;
};

export type LoadOptions = {
  force?: boolean;
  until?: (entries: BrowseEntry[]) => boolean;
};

export type LoadResult = DirectoryState & { settled: boolean };

export type DirectoryCache = {
  get: (path: string) => DirectoryState;
  ensure: (path: string) => Promise<LoadResult>;
  load: (path: string, options?: LoadOptions) => Promise<LoadResult>;
  refresh: (
    path: string,
    until?: (entries: BrowseEntry[]) => boolean,
  ) => Promise<LoadResult>;
  invalidate: (path: string) => void;
  invalidateSubtree: (path: string) => void;
  subscribe: (listener: () => void) => () => void;
};

const FRESH_MS = 60_000;
const REFRESH_ATTEMPTS = 6;
const REFRESH_INTERVAL_MS = 250;

const EMPTY_STATE: DirectoryState = {
  entries: [],
  loading: false,
  error: "",
  updatedAt: 0,
};

function normalize(path: string) {
  const trimmed = path.replace(/\/+$/, "");
  return trimmed.startsWith("/") ? trimmed || "/" : `/${trimmed}`;
}

function wait(ms: number) {
  return new Promise<void>((resolve) => {
    setTimeout(resolve, ms);
  });
}

export function createDirectoryCache(): DirectoryCache {
  const states = new Map<string, DirectoryState>();
  const flights = new Map<string, Promise<LoadResult>>();
  const sequences = new Map<string, number>();
  const listeners = new Set<() => void>();
  let sequence = 0;

  const emit = () => {
    for (const listener of listeners) listener();
  };

  const get = (raw: string) => states.get(normalize(raw)) ?? EMPTY_STATE;

  const patch = (path: string, next: Partial<DirectoryState>) => {
    states.set(path, { ...(states.get(path) ?? EMPTY_STATE), ...next });
    emit();
  };

  const load = (raw: string, options: LoadOptions = {}): Promise<LoadResult> => {
    const path = normalize(raw);
    const running = flights.get(path);
    if (running && !options.force) return running;

    const requestId = ++sequence;
    sequences.set(path, requestId);
    patch(path, { loading: true, error: "" });

    const task = (async (): Promise<LoadResult> => {
      try {
        let result = await browse(path);
        for (
          let attempt = 0;
          attempt < REFRESH_ATTEMPTS &&
          options.until &&
          !options.until(result.entries);
          attempt++
        ) {
          await wait(REFRESH_INTERVAL_MS);
          if (sequences.get(path) !== requestId) {
            return { ...get(path), settled: false };
          }
          result = await browse(path);
        }
        if (sequences.get(path) !== requestId) {
          return { ...get(path), settled: false };
        }
        const settled = options.until ? options.until(result.entries) : true;
        patch(path, {
          entries: result.entries,
          loading: false,
          error: "",
          updatedAt: Date.now(),
        });
        return { ...get(path), settled };
      } catch (cause) {
        if (sequences.get(path) !== requestId) {
          return { ...get(path), settled: false };
        }
        patch(path, {
          loading: false,
          error: cause instanceof Error ? cause.message : "读取目录失败",
        });
        return { ...get(path), settled: false };
      }
    })();

    const tracked = task.finally(() => {
      if (flights.get(path) === tracked) flights.delete(path);
    });
    flights.set(path, tracked);
    return tracked;
  };

  const ensure = async (raw: string): Promise<LoadResult> => {
    const path = normalize(raw);
    const cached = states.get(path);
    if (!cached) return load(path);
    if (Date.now() - cached.updatedAt < FRESH_MS) {
      return { ...cached, settled: true };
    }
    void load(path);
    return { ...cached, settled: true };
  };

  const refresh = (raw: string, until?: (entries: BrowseEntry[]) => boolean) =>
    load(raw, { force: true, until });

  const invalidate = (raw: string) => {
    const path = normalize(raw);
    sequences.delete(path);
    flights.delete(path);
    if (states.delete(path)) emit();
  };

  const invalidateSubtree = (raw: string) => {
    const path = normalize(raw);
    const prefix = path === "/" ? "/" : `${path}/`;
    let changed = false;
    for (const key of [...states.keys()]) {
      if (key !== path && !key.startsWith(prefix)) continue;
      states.delete(key);
      sequences.delete(key);
      flights.delete(key);
      changed = true;
    }
    if (changed) emit();
  };

  const subscribe = (listener: () => void) => {
    listeners.add(listener);
    return () => {
      listeners.delete(listener);
    };
  };

  return {
    get,
    ensure,
    load,
    refresh,
    invalidate,
    invalidateSubtree,
    subscribe,
  };
}

export function useDirectoryCache(): DirectoryCache {
  const [cache] = useState(createDirectoryCache);
  return cache;
}

export function useDirectoryState(cache: DirectoryCache, path: string) {
  const getSnapshot = useCallback(() => cache.get(path), [cache, path]);
  return useSyncExternalStore(cache.subscribe, getSnapshot);
}
