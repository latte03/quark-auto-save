import { useCallback, useEffect, useRef, useState } from "react";
import { getSession, logout, setUnauthorizedHandler, type SessionInfo } from "../lib/api";

const EXPIRED_NOTICE = "登录状态已过期，请重新登录";

export function useConsoleSession() {
  const [session, setSession] = useState<SessionInfo | null>(null);
  const [bootError, setBootError] = useState("");
  const [expiredNotice, setExpiredNotice] = useState("");
  const checking = useRef(false);

  const markExpired = useCallback(() => {
    setExpiredNotice(EXPIRED_NOTICE);
    setSession((prev) => ({
      authenticated: false,
      username: prev?.username ?? "",
      version: prev?.version ?? "",
    }));
  }, []);

  // Any 401 — REST or the run stream — means the cookie is gone; ask the
  // server once, then fall back to the login form instead of a dead console.
  const revalidate = useCallback(() => {
    if (checking.current) return;
    checking.current = true;
    void getSession()
      .then((info) => {
        if (!info.authenticated) markExpired();
      })
      .catch(() => undefined)
      .finally(() => {
        window.setTimeout(() => {
          checking.current = false;
        }, 1000);
      });
  }, [markExpired]);

  useEffect(() => {
    setUnauthorizedHandler(revalidate);
    return () => setUnauthorizedHandler(null);
  }, [revalidate]);

  const signOut = useCallback(async () => {
    try {
      await logout();
    } catch {
      // The cookie is cleared server-side anyway on the next login.
    }
    setSession((prev) => ({
      authenticated: false,
      username: "",
      version: prev?.version ?? "",
    }));
  }, []);

  return {
    session,
    setSession,
    bootError,
    setBootError,
    expiredNotice,
    markExpired,
    revalidate,
    signOut,
  };
}
