#!/usr/bin/env python3
"""Generate golden cases for the Go magic-regex parity test.

Runs the same two rules the Go port must match through CPython's `re`,
using the patterns from the repository's quark_config.json.

    python3 poc/tools/gen_magic_cases.py
"""

import json
import re
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
OUT = ROOT / "poc" / "quark" / "testdata" / "magic_cases.json"
RULES = ("$BLACK_WORD", "$TV_REGEX")

FILES = [
    "黑镜 S07E05.mkv",
    "Black.Mirror.S07E05.mkv",
    "Show.第12集.mp4",
    "Show.第 7 集.mp4",
    "Show.第03集.mp4",
    "Show.E03.mkv",
    "Show.EP04.mkv",
    "Show.S01E01.mkv",
    "Show.S7E12.mp4",
    "Show.E5.mkv",
    "Show.第100集.mp4",
    "Show.1080p.mkv",
    "Show.2024.05.01.mp4",
    "movie.mp4",
    "2024.01.01.mkv",
    "黑镜.第6集.1080p.mkv",
    "[纯享版] 黑镜 S07E05.mkv",
    "某某 加更 第3期.mp4",
    "超前企划 先导片.mp4",
    "训练室 花絮.mkv",
    "蒸蒸日上 第2期.mkv",
    "漫长的季节 第12集.mp4",
    "纯享 加更 超前企划 训练室 蒸蒸日上.mp4",
    "",
]


def main() -> None:
    config = json.loads((ROOT / "quark_config.json").read_text(encoding="utf-8"))
    rules = config["magic_regex"]
    cases = []
    for name in RULES:
        pattern = rules[name]["pattern"]
        replace = rules[name]["replace"]
        for filename in FILES:
            matched = re.search(pattern, filename) is not None
            result = re.sub(pattern, replace, filename) if replace else filename
            cases.append(
                {
                    "rule": name,
                    "pattern": pattern,
                    "replace": replace,
                    "filename": filename,
                    "matched": matched,
                    "result": result,
                }
            )
    OUT.parent.mkdir(parents=True, exist_ok=True)
    OUT.write_text(
        json.dumps({"cases": cases}, ensure_ascii=False, indent=2) + "\n",
        encoding="utf-8",
    )
    print(f"wrote {len(cases)} cases to {OUT.relative_to(ROOT)}")


if __name__ == "__main__":
    main()
