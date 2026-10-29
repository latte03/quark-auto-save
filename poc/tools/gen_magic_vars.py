# -*- coding: utf-8 -*-
"""Generate Go parity fixtures for MagicRename variables and {I+} sorting.

Run with the parity venv:  poc/.venv/bin/python poc/tools/gen_magic_vars.py
"""
import json
import os
import sys
from datetime import datetime

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
sys.path.insert(0, ROOT)

import quark_auto_save as qas  # noqa: E402

TASKNAME = "测试任务"
YEAR = datetime.now().year

REPLACES = [
    "{TASKNAME}.{SXX}E{E}.{EXT}",
    "{TASKNAME}.{SXX}E{II}.第{E}期{PART}.{EXT}",
    "{YEAR}.{DATE}.{CHINESE}.{EXT}",
    "{S}-{E}-{PART}-{VER}.{EXT}",
    "{EXT}",
    "{SXX}",
    "{E}",
    "{PART}",
    "{S}",
    "{DATE}",
    "{YEAR}",
    "{CHINESE}",
    "{TASKNAME}",
    "{UNKNOWN}.{EXT}",
]

FILENAMES = [
    "Show.S01E05.1080p.mkv",
    "Show.2024.05.01.mp4",
    "2024.01.01.mkv",
    "第03集.mp4",
    "20240501.mkv",
    "05-01.mp4",
    "某综艺 第12期 上 4K.mp4",
    "某综艺 第十期 下.mp4",
    "电影.2019.国语版.mkv",
    "Show.EP07.mkv",
    "Show_08_1080p.mkv",
    "09.mkv",
    "第2期 中.mp4",
    "无扩展名",
    ".hidden",
    "剧名.第5集.超前企划.mp4",
    "A.1.2.3.mkv",
    "Show.S2E10.mkv",
    "Show.1080p.mkv",
    "x.mp4",
]


def sub_cases():
    cases = []
    for replace in REPLACES:
        for filename in FILENAMES:
            mr = qas.MagicRename()
            mr.set_taskname(TASKNAME)
            result = mr.sub(r"^.*$", replace, filename)
            cases.append(
                {
                    "replace": replace,
                    "pattern": r"^.*$",
                    "filename": filename,
                    "result": result,
                }
            )
    return cases


def dir_cases():
    cases = []

    def share(name, updated_at, directory=False):
        return {"file_name": name, "dir": directory, "updated_at": updated_at}

    scenarios = [
        {
            "replace": "{TASKNAME}.{I}.{EXT}",
            "dir": ["任务.01.mkv", "任务.03.mkv", "无关.mkv"],
            "incoming": [
                ("甲.mkv", 1000),
                ("乙.mp4", 2000),
                ("丙.mkv", 3000),
            ],
        },
        {
            "replace": "{TASKNAME}.{II}.{EXT}",
            "dir": ["任务.01.mkv", "任务.02.mkv"],
            "incoming": [("新1.mkv", 1000), ("新2.mkv", 2000)],
        },
        {
            "replace": "第{I}期.{EXT}",
            "dir": ["第01期.mp4", "第03期.mp4"],
            "incoming": [("a.mp4", 100), ("b.mp4", 200)],
        },
        {
            "replace": "{TASKNAME}.{I}.{EXT}",
            "dir": [],
            "incoming": [("x.mkv", 300), ("y.mkv", 100), ("z.mkv", 200)],
        },
        {
            "replace": "{TASKNAME}.{I}.{EXT}",
            "dir": ["任务.10.mkv"],
            "incoming": [("x.mkv", 100)],
        },
        {
            "replace": "{TASKNAME}.第{I}期.第{E}期.{EXT}",
            "dir": [],
            "incoming": [
                ("某综艺 第2期 上.mp4", 100),
                ("某综艺 第10期 下.mp4", 200),
            ],
        },
    ]

    for scenario in scenarios:
        mr = qas.MagicRename()
        mr.set_taskname(TASKNAME)
        replace = scenario["replace"]
        dir_files = [share(name, 0) for name in scenario["dir"]]
        mr.set_dir_file_list(dir_files, replace)
        pattern, repl = mr.magic_regex_conv("", replace)
        incoming = []
        for name, updated_at in scenario["incoming"]:
            file_name_re = mr.sub("^.*$", replace, name)
            incoming.append(
                {
                    "file_name": name,
                    "file_name_re_in": file_name_re,
                    "file_name_re": file_name_re,
                    "updated_at": updated_at,
                    "dir": False,
                }
            )
        mr.sort_file_list(incoming)
        cases.append(
            {
                "taskname": TASKNAME,
                "replace": replace,
                "pattern": pattern,
                "dir_files": dir_files,
                "incoming": incoming,
                "dir_after": {str(k): v for k, v in sorted(mr.dir_filename_dict.items())},
            }
        )
    return cases


def exists_cases():
    cases = []
    scenarios = [
        ("Show.E01.mkv", ["Show.E01.mkv", "Other.mkv"], False),
        ("Show.E01.mkv", ["Show.E01.MKV"], False),
        ("Show.E01.mkv", ["Show.E01.MKV"], True),
        ("Show.E01.mkv", ["Show.E02.mkv"], True),
        ("Show.{I}.mkv", ["Show.01.mkv", "Show.02.mkv"], False),
        ("Show.{II}.mkv", ["Show.05.mkv"], False),
        ("Show.{I}.mkv", ["Show.5.mkv", "Show.05.mkv"], False),
        ("第{I}期.mp4", ["第07期.mp4"], False),
        ("archive.tar.gz", ["archive.tar.rar"], True),
        (".hidden", [".hidden"], False),
        ("a.mkv", [".a.mkv"], True),
    ]
    for filename, filenames, ignore_ext in scenarios:
        mr = qas.MagicRename()
        cases.append(
            {
                "filename": filename,
                "filenames": filenames,
                "ignore_ext": ignore_ext,
                "result": mr.is_exists(filename, filenames, ignore_ext) or "",
            }
        )
    return cases


def sort_cases():
    names = [
        "任务.1.mkv_1000",
        "任务.10.mkv_2000",
        "任务.2.mkv_500",
        "任务.02.mkv_300",
        "任务.S1E10.mkv_1",
        "任务.S1E2.mkv_2",
        "任务.第2期.第10集.mp4_1",
        "任务.第2期.第9集.mp4_2",
        "任务.上.mp4_1",
        "任务.下.mp4_2",
        "任务.中.mp4_3",
        "任务.一百.第3集.mp4_4",
        "任务.a.mp4_1",
        "任务.B.mp4_2",
        "任务.1a.mp4_1",
        "任务.1.5.mp4_2",
        "任务.-1.mp4_3",
        "任务.001.mp4_4",
        "任务.1.mp4_5",
    ]
    mr = qas.MagicRename()
    ordered = qas.natsorted(names, key=mr._custom_sort_key)
    return [{"names": names, "want": ordered}]


def main():
    payload = {
        "sort": sort_cases(),
        "taskname": TASKNAME,
        "year": YEAR,
        "sub": sub_cases(),
        "dir": dir_cases(),
        "exists": exists_cases(),
    }
    out = os.path.join(ROOT, "poc", "quark", "testdata", "magic_vars.json")
    with open(out, "w", encoding="utf-8") as f:
        json.dump(payload, f, ensure_ascii=False, indent=2)
    print(f"wrote {out}: {len(payload['sub'])} sub, {len(payload['dir'])} dir, {len(payload['exists'])} exists")


if __name__ == "__main__":
    main()
