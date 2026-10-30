import { useCallback, useState } from "react";

/** Runs an async UI action behind a `busy` flag; errors stay with the caller. */
export function useAsyncAction() {
  const [busy, setBusy] = useState(false);
  const run = useCallback(async (task: () => Promise<void>) => {
    setBusy(true);
    try {
      await task();
    } finally {
      setBusy(false);
    }
  }, []);
  return [busy, run] as const;
}
