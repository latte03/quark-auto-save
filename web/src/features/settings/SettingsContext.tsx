import {
  createContext,
  useCallback,
  useContext,
  useMemo,
  type ReactNode,
  type SetStateAction,
} from "react";
import type { AppConfig, PluginConfig } from "../../lib/types";
import type { ThemeMode } from "../../lib/theme";
import { useSourceDetection, type Detection } from "./useSourceDetection";

type SettingsContextValue = {
  config: AppConfig;
  setConfig: (updater: SetStateAction<AppConfig>) => void;
  update: <K extends keyof AppConfig>(key: K, value: AppConfig[K]) => void;
  updatePlugin: (
    plugin: string,
    key: string,
    value: PluginConfig[string],
  ) => void;
  updateSource: (
    source: "cloudsaver" | "pansou",
    key: string,
    value: string | boolean,
  ) => void;
  updateLocalDownload: (
    key: keyof AppConfig["local_download"],
    value: string | number | boolean,
  ) => void;
  updateMagic: (
    id: string,
    field: "name" | "pattern" | "replace",
    value: string,
  ) => void;
  pluginStatus: Record<string, boolean>;
  running: boolean;
  theme: ThemeMode;
  onThemeChange: (mode: ThemeMode) => void;
  onAdoptDefaults: () => void;
  onSign: () => void;
  onTasksReload: () => void;
  detection: Detection;
};

const SettingsContext = createContext<SettingsContextValue | null>(null);

export type SettingsProviderProps = {
  config: AppConfig;
  setConfig: (updater: SetStateAction<AppConfig>) => void;
  pluginStatus: Record<string, boolean>;
  running: boolean;
  theme: ThemeMode;
  onThemeChange: (mode: ThemeMode) => void;
  onAdoptDefaults: () => void;
  onSign: () => void;
  onTasksReload: () => void;
  children: ReactNode;
};

export function SettingsProvider({
  config,
  setConfig,
  pluginStatus,
  running,
  theme,
  onThemeChange,
  onAdoptDefaults,
  onSign,
  onTasksReload,
  children,
}: SettingsProviderProps) {
  const update = useCallback(
    <K extends keyof AppConfig,>(key: K, value: AppConfig[K]) =>
      setConfig((prev) => ({ ...prev, [key]: value })),
    [setConfig],
  );

  const updatePlugin = useCallback(
    (plugin: string, key: string, value: PluginConfig[string]) =>
      setConfig((prev) => ({
        ...prev,
        plugins: {
          ...prev.plugins,
          [plugin]: { ...prev.plugins[plugin], [key]: value },
        },
      })),
    [setConfig],
  );

  const updateLocalDownload = useCallback(
    (key: keyof AppConfig["local_download"], value: string | number | boolean) =>
      setConfig((prev) => ({
        ...prev,
        local_download: { ...prev.local_download, [key]: value },
      })),
    [setConfig],
  );

  const updateSource = useCallback(
    (source: "cloudsaver" | "pansou", key: string, value: string | boolean) =>
      setConfig((prev) => ({
        ...prev,
        source: {
          ...prev.source,
          [source]: { ...prev.source[source], [key]: value },
        },
      })),
    [setConfig],
  );

  const updateMagic = useCallback(
    (id: string, field: "name" | "pattern" | "replace", value: string) =>
      setConfig((prev) => ({
        ...prev,
        magic_regex: prev.magic_regex.map((rule) =>
          rule.id === id ? { ...rule, [field]: value } : rule,
        ),
      })),
    [setConfig],
  );

  const detection = useSourceDetection({
    config,
    setConfig,
    updateSource,
    onAdoptDefaults,
  });

  const value = useMemo<SettingsContextValue>(
    () => ({
      config,
      setConfig,
      update,
      updatePlugin,
      updateSource,
      updateLocalDownload,
      updateMagic,
      pluginStatus,
      running,
      theme,
      onThemeChange,
      onAdoptDefaults,
      onSign,
      onTasksReload,
      detection,
    }),
    [
      config,
      setConfig,
      update,
      updatePlugin,
      updateSource,
      updateLocalDownload,
      updateMagic,
      pluginStatus,
      running,
      theme,
      onThemeChange,
      onAdoptDefaults,
      onSign,
      onTasksReload,
      detection,
    ],
  );

  return (
    <SettingsContext.Provider value={value}>{children}</SettingsContext.Provider>
  );
}

export function useSettings() {
  const value = useContext(SettingsContext);
  if (!value) {
    throw new Error("useSettings must be used inside SettingsProvider");
  }
  return value;
}
