import { useCallback, useEffect, useRef, useState, type SetStateAction } from "react";
import { saveConfig } from "../lib/api";
import type { AppConfig } from "../lib/types";
import { deepEqual } from "../lib/compare";

export function useSettingsDraft() {
  const [config, setConfig] = useState<AppConfig | null>(null);
  const [savedConfig, setSavedConfig] = useState<AppConfig | null>(null);
  const [adoptTick, setAdoptTick] = useState(0);
  const adoptedTick = useRef(0);

  const load = useCallback((next: AppConfig) => {
    setConfig(next);
    setSavedConfig(next);
  }, []);

  const clear = useCallback(() => {
    setConfig(null);
    setSavedConfig(null);
  }, []);

  const update = useCallback((updater: SetStateAction<AppConfig>) => {
    setConfig((prev) => {
      if (!prev) return prev;
      return typeof updater === "function" ? updater(prev) : updater;
    });
  }, []);

  const dirty = Boolean(config && savedConfig && !deepEqual(config, savedConfig));

  const save = useCallback(async () => {
    const snapshot = config;
    if (!snapshot) return;
    const saved = await saveConfig({
      cookie: snapshot.cookie,
      crontab: snapshot.crontab,
      push_config: snapshot.push_config,
      plugins: snapshot.plugins,
      magic_regex: snapshot.magic_regex,
      source: snapshot.source,
      local_download: snapshot.local_download,
      run_logs: snapshot.run_logs,
    });
    setSavedConfig(saved);
    // 保存期间用户可能继续编辑：草稿变了就保留草稿，但服务端重算的派生字段仍要跟上。
    setConfig((prev) =>
      !prev || deepEqual(prev, snapshot)
        ? saved
        : { ...prev, next_runs: saved.next_runs, version: saved.version },
    );
  }, [config]);

  // Values the settings page fills in on its own (download dir, detected
  // sources) are not unsaved edits, so the baseline absorbs them.
  const adoptDefaults = useCallback(() => setAdoptTick((tick) => tick + 1), []);

  // Same idea for a patch the server already persisted (e.g. QR login writes
  // cookies): update both sides so the page does not claim unsaved changes.
  const adoptPatch = useCallback((patch: Partial<AppConfig>) => {
    setConfig((prev) => (prev ? { ...prev, ...patch } : prev));
    setSavedConfig((prev) => (prev ? { ...prev, ...patch } : prev));
  }, []);

  useEffect(() => {
    if (adoptTick === adoptedTick.current) return;
    adoptedTick.current = adoptTick;
    setSavedConfig(config);
  }, [adoptTick, config]);

  return {
    config,
    savedConfig,
    dirty,
    load,
    clear,
    update,
    save,
    adoptDefaults,
    adoptPatch,
  };
}
