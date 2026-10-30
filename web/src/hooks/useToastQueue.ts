import { useCallback, useEffect, useRef, useState } from "react";
import type { ToastData, ToastTone } from "../components/Toast";

export function useToastQueue() {
  const [toast, setToast] = useState<ToastData | null>(null);
  const timer = useRef<number | null>(null);

  const notify = useCallback((message: string, tone: ToastTone = "info") => {
    setToast({ id: Date.now(), message, tone });
    if (timer.current) window.clearTimeout(timer.current);
    timer.current = null;
    // Failures stay until dismissed; success and hints fade on their own.
    if (tone !== "error") {
      timer.current = window.setTimeout(() => setToast(null), 3200);
    }
  }, []);

  useEffect(
    () => () => {
      if (timer.current) window.clearTimeout(timer.current);
    },
    [],
  );

  return { toast, notify, dismiss: () => setToast(null) };
}
