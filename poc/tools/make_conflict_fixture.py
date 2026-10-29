# -*- coding: utf-8 -*-
"""Derive a conflict fixture from the captured real-task fixture.

It keeps the real share snapshots but fills destination directories with
files that already exist, so skip / ignore-extension / {I} continuation and
update_subdir recursion are compared against Python as well.
"""
import copy
import json
import os
import sys

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
SRC = os.path.join(ROOT, "poc", "quark", "testdata", "real_tasks.json")
DST = os.path.join(ROOT, "poc", "quark", "testdata", "real_tasks_conflicts.json")

ROOT_FID = "7e25ddd87cf64443b637125478733295"
SHOW_FID = "71df3902f42d4270a58c0eb12aa2b014"
AD_FID = "680d91e490814da0927c38b432f88edc"


def file_entry(fid, name, directory=False, updated_at=1):
    return {
        "fid": fid,
        "share_fid_token": "",
        "file_name": name,
        "dir": directory,
        "size": 0,
        "updated_at": updated_at,
        "obj_category": "",
        "include_items": 0,
    }


def main():
    with open(SRC, encoding="utf-8") as f:
        fixture = json.load(f)
    fixture = copy.deepcopy(fixture)
    fixture["notes"] = (
        "由 real_tasks.json 派生：目标目录预置已存在文件，"
        "用于对照跳过、忽略后缀、{I} 续号与 update_subdir 递归"
    )
    tasks = []
    for task in fixture["tasks"][:3]:
        tasks.append(task)

    root_task = copy.deepcopy(fixture["tasks"][0])
    show_task = copy.deepcopy(fixture["tasks"][1])
    ad_task = copy.deepcopy(fixture["tasks"][2])

    recursion = {
        "taskname": "冲突-递归子目录",
        "shareurl": root_task["shareurl"],
        "savepath": "/夸克自动转存测试/冲突",
        "pattern": show_task["pattern"],
        "replace": show_task["replace"],
        "update_subdir": "综艺命名",
    }
    ignore_ext = {
        "taskname": "冲突-忽略后缀",
        "shareurl": ad_task["shareurl"],
        "savepath": "/夸克自动转存测试/冲突忽略",
        "pattern": ad_task["pattern"],
        "replace": ad_task["replace"],
        "ignore_extension": True,
    }
    numbering = {
        "taskname": "冲突-编号续接",
        "shareurl": show_task["shareurl"],
        "savepath": "/夸克自动转存测试/冲突编号",
        "pattern": show_task["pattern"],
        "replace": show_task["replace"],
    }
    tasks.extend([recursion, ignore_ext, numbering])

    existing_show = [
        "01.测试-综艺命名.20250321.第1期上.mp4",
        "02.测试-综艺命名.20250321.第1期中.mp4",
        "2025-03-21 第1期下：一公分组评级小考 4K.mp4",
    ]
    fixture["dest"]["/夸克自动转存测试/冲突"] = [
        file_entry("dest-dir-show", "综艺命名", True),
    ]
    fixture["dest"]["/夸克自动转存测试/冲突/综艺命名"] = [
        file_entry("dest-existing-%d" % i, name) for i, name in enumerate(existing_show)
    ]
    fixture["dest"]["/夸克自动转存测试/冲突忽略"] = [
        file_entry("dest-ad-nfo", "剧名.03.nfo"),
    ]
    fixture["dest"]["/夸克自动转存测试/冲突编号"] = [
        file_entry("dest-num-1", "01.测试-综艺命名.20250321.第1期上.mp4"),
        file_entry("dest-num-2", "02.测试-综艺命名.20250321.第1期中.mp4"),
    ]
    fixture["tasks"] = tasks

    with open(DST, "w", encoding="utf-8") as f:
        json.dump(fixture, f, ensure_ascii=False, indent=2)
        f.write("\n")
    print("wrote %s: %d tasks, %d dest dirs" % (DST, len(tasks), len(fixture["dest"])))


if __name__ == "__main__":
    main()
