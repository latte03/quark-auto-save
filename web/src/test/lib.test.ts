import { describe, expect, it } from "vitest";
import { formatBytes } from "../lib/mock";
import { resolveDark, themeLabels, type ThemeMode } from "../lib/theme";

describe("formatBytes", () => {
  it("hides empty sizes", () => {
    expect(formatBytes(0)).toBe("—");
  });

  it("scales into the largest fitting unit", () => {
    expect(formatBytes(512)).toBe("512 B");
    expect(formatBytes(2048)).toBe("2.0 KB");
    expect(formatBytes(1024 * 1024 * 3.5)).toBe("3.5 MB");
    expect(formatBytes(1024 ** 3 * 12)).toBe("12.0 GB");
  });

  it("drops decimals from three-digit values", () => {
    expect(formatBytes(1024 * 120)).toBe("120 KB");
  });
});

describe("theme", () => {
  it("follows the system preference in system mode", () => {
    expect(resolveDark("system", true)).toBe(true);
    expect(resolveDark("system", false)).toBe(false);
  });

  it("pins the explicit modes", () => {
    expect(resolveDark("dark", false)).toBe(true);
    expect(resolveDark("light", true)).toBe(false);
  });

  it("labels every mode", () => {
    const modes: ThemeMode[] = ["system", "light", "dark"];
    for (const mode of modes) expect(themeLabels[mode]).toBeTruthy();
  });
});
