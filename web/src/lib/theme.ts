export type ThemeMode = "system" | "light" | "dark";

const STORAGE_KEY = "qas-theme";

export const themeLabels: Record<ThemeMode, string> = {
  system: "跟随系统",
  light: "浅色",
  dark: "深色",
};

export function readThemeMode(): ThemeMode {
  try {
    const stored = window.localStorage.getItem(STORAGE_KEY);
    if (stored === "light" || stored === "dark" || stored === "system") {
      return stored;
    }
  } catch {
    // Storage can be blocked; fall back to the system preference.
  }
  return "system";
}

export function resolveDark(mode: ThemeMode, systemPrefersDark: boolean): boolean {
  if (mode === "system") return systemPrefersDark;
  return mode === "dark";
}

export function systemPrefersDark(): boolean {
  return window.matchMedia?.("(prefers-color-scheme: dark)").matches ?? false;
}

export function applyTheme(
  mode: ThemeMode,
  systemDark = systemPrefersDark(),
): boolean {
  const dark = resolveDark(mode, systemDark);
  document.documentElement.classList.toggle("dark", dark);
  return dark;
}

export function saveThemeMode(mode: ThemeMode) {
  try {
    window.localStorage.setItem(STORAGE_KEY, mode);
  } catch {
    // Persisting is best-effort; the session still switches.
  }
}
