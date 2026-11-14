import { useCallback, useEffect, useState } from "react";
import type { Page } from "../lib/types";

export type Route = { page: Page; runId: string | null };

const PAGES: Page[] = ["overview", "tasks", "files", "runs", "settings"];

function parseHash(): Route {
  if (typeof window === "undefined") return { page: "overview", runId: null };
  const raw = window.location.hash.replace(/^#\/?/, "");
  if (!raw) return { page: "overview", runId: null };
  const [head, ...rest] = raw.split("/");
  const page = PAGES.find((item) => item === head) ?? "overview";
  const runId = page === "runs" && rest.length > 0 ? decodeURIComponent(rest.join("/")) : null;
  return { page, runId };
}

function toHash(page: Page, runId?: string | null): string {
  if (page === "overview" && !runId) return "";
  const suffix = page === "runs" && runId ? `/${encodeURIComponent(runId)}` : "";
  return `#/${page}${suffix}`;
}

/**
 * 页面状态与 URL hash 双向同步：刷新保持当前页、可收藏、
 * `#/runs/<id>` 可直达一次运行的日志。
 */
export function useHashRoute() {
  const [route, setRoute] = useState<Route>(parseHash);

  useEffect(() => {
    const onHashChange = () => setRoute(parseHash());
    window.addEventListener("hashchange", onHashChange);
    return () => window.removeEventListener("hashchange", onHashChange);
  }, []);

  const navigate = useCallback((page: Page, runId?: string | null) => {
    const target = toHash(page, runId);
    if (window.location.hash === target || (!target && !window.location.hash)) {
      setRoute(parseHash());
      return;
    }
    // 写 hash 触发 hashchange → setRoute；后退/前进同样生效。
    window.location.hash = target || "#/";
  }, []);

  /** 直达一次运行的日志后清除 runId 段，避免重开弹窗；不触发 hashchange。 */
  const consumeRunId = useCallback(() => {
    if (!window.location.hash.includes("/")) return;
    const page = parseHash().page;
    window.history.replaceState(null, "", toHash(page) || window.location.pathname);
    setRoute((prev) => ({ ...prev, runId: null }));
  }, []);

  const reset = useCallback(() => {
    window.history.replaceState(null, "", window.location.pathname + window.location.search);
    setRoute({ page: "overview", runId: null });
  }, []);

  return { route, navigate, consumeRunId, reset };
}
