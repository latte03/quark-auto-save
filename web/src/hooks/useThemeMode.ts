import { useCallback, useEffect, useState } from "react";
import {
  applyTheme,
  readThemeMode,
  saveThemeMode,
  type ThemeMode,
} from "../lib/theme";

export function useThemeMode() {
  const [theme, setTheme] = useState<ThemeMode>(readThemeMode);

  useEffect(() => {
    applyTheme(theme);
  }, [theme]);

  useEffect(() => {
    if (theme !== "system") return;
    const query = window.matchMedia("(prefers-color-scheme: dark)");
    const onChange = () => applyTheme("system", query.matches);
    query.addEventListener("change", onChange);
    return () => query.removeEventListener("change", onChange);
  }, [theme]);

  const changeTheme = useCallback((mode: ThemeMode) => {
    saveThemeMode(mode);
    setTheme(mode);
  }, []);

  return { theme, changeTheme };
}
