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
    if (!config) return;
    const saved = await saveConfig({
      cookie: config.cookie,
      crontab: config.crontab,
      push_config: config.push_config,
      plugins: config.plugins,
      magic_regex: config.magic_regex,
      source: config.source,
      local_download: config.local_download,
      run_logs: config.run_logs,
    });
    setConfig(saved);
    setSavedConfig(saved);
  }, [config]);

  // Values the settings page fills in on its own (download dir, detected
  // sources) are not unsaved edits, so the baseline absorbs them.
  const adoptDefaults = useCallback(() => setAdoptTick((tick) => tick + 1), []);

  useEffect(() => {
    if (adoptTick === adoptedTick.current) return;
    adoptedTick.current = adoptTick;
    setSavedConfig(config);
  }, [adoptTick, config]);

  return { config, savedConfig, dirty, load, clear, update, save, adoptDefaults };
}
