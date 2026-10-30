import { describe, expect, it } from "vitest";
import { nextRunRank } from "../lib/schedule";

// 2026-10-30 is a Friday, so 周五 must rank a full week out, not "now".
const friday = new Date(2026, 9, 30, 10, 0, 0);

describe("nextRunRank", () => {
  it("pushes unparseable labels to the end", () => {
    expect(nextRunRank("")).toBe(Number.POSITIVE_INFINITY);
    expect(nextRunRank("—")).toBe(Number.POSITIVE_INFINITY);
    expect(nextRunRank("明天 08:00")).toBe(Number.POSITIVE_INFINITY);
    expect(nextRunRank("周六 8点")).toBe(Number.POSITIVE_INFINITY);
  });

  it("ranks today by minutes after midnight", () => {
    expect(nextRunRank("今天 10:30", friday)).toBe(10 * 60 + 30);
    expect(nextRunRank("今天 00:00", friday)).toBe(0);
  });

  it("ranks the remaining weekdays by distance from now", () => {
    expect(nextRunRank("周六 08:00", friday)).toBe(24 * 60 + 8 * 60);
    expect(nextRunRank("周日 08:00", friday)).toBe(2 * 24 * 60 + 8 * 60);
    expect(nextRunRank("周四 08:00", friday)).toBe(6 * 24 * 60 + 8 * 60);
  });

  it("treats today's weekday label as next week", () => {
    expect(nextRunRank("周五 08:00", friday)).toBe(7 * 24 * 60 + 8 * 60);
  });

  it("sorts today ahead of later weekdays", () => {
    const labels = ["周一 06:00", "今天 22:00", "周六 01:00"];
    const sorted = [...labels].sort((a, b) => nextRunRank(a, friday) - nextRunRank(b, friday));
    expect(sorted).toEqual(["今天 22:00", "周六 01:00", "周一 06:00"]);
  });
});
