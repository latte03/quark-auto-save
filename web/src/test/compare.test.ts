import { describe, expect, it } from "vitest";
import { deepEqual } from "../lib/compare";

describe("deepEqual", () => {
  it("treats primitives and identical references as equal", () => {
    expect(deepEqual(1, 1)).toBe(true);
    expect(deepEqual("a", "a")).toBe(true);
    expect(deepEqual(null, null)).toBe(true);
    expect(deepEqual(Number.NaN, Number.NaN)).toBe(true);
    expect(deepEqual(1, "1")).toBe(false);
  });

  it("ignores object key order", () => {
    expect(deepEqual({ a: 1, b: 2 }, { b: 2, a: 1 })).toBe(true);
    expect(deepEqual({ a: 1 }, { a: 1, b: undefined })).toBe(false);
  });

  it("compares arrays by position", () => {
    expect(deepEqual([1, 2], [1, 2])).toBe(true);
    expect(deepEqual([1, 2], [2, 1])).toBe(false);
    expect(deepEqual([1], [1, 2])).toBe(false);
  });

  it("walks nested config shapes", () => {
    const saved = {
      cookie: ["a=1"],
      local_download: { dir: "/vol1/media", concurrency: 2 },
      plugins: { aria2: { enable: false } },
    };
    const edited = {
      plugins: { aria2: { enable: false } },
      local_download: { concurrency: 2, dir: "/vol1/media" },
      cookie: ["a=1"],
    };
    expect(deepEqual(saved, edited)).toBe(true);
    expect(
      deepEqual(saved, {
        ...saved,
        local_download: { ...saved.local_download, concurrency: 3 },
      }),
    ).toBe(false);
  });

  it("does not confuse null and missing objects", () => {
    expect(deepEqual(null, {})).toBe(false);
    expect(deepEqual({ a: null }, {})).toBe(false);
  });
});
