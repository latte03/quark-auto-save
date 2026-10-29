# -*- coding: utf-8 -*-
"""Run the upstream Python pipeline against a captured fixture and write the plan.

Usage:
  poc/.venv/bin/python poc/tools/parity_plan.py [fixture] [out]

The fixture comes from `qaspoc dump`; output is compared by
quark/pipeline_fixture_test.go.
"""
import contextlib
import copy
import json
import os
import sys

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
sys.path.insert(0, ROOT)

import quark_auto_save as qas  # noqa: E402


def dest_fid(path):
    return "dest:" + path


class FakeQuark(qas.Quark):
    def __init__(self, share, dest):
        super().__init__("fake")
        self.share = share
        self.dest = dest
        self.renames = []
        self.top = 0

    def get_stoken(self, pwd_id, passcode=""):
        return {"status": 200, "code": 0, "data": {"stoken": "fake", "title": "fixture"}}

    def get_detail(self, pwd_id, stoken, pdir_fid, _fetch_share=0, fetch_share_full_path=0):
        return {"code": 0, "data": {"list": copy.deepcopy(self.share.get(str(pdir_fid), []))}}

    def get_fids(self, file_paths):
        return [{"file_path": p, "fid": dest_fid(p)} for p in file_paths if p in self.dest]

    def ls_dir(self, pdir_fid, page=1, size=50):
        path = str(pdir_fid)
        if path.startswith("dest:"):
            path = path[len("dest:"):]
        return {"code": 0, "data": {"list": copy.deepcopy(self.dest.get(path, []))}}

    def save_file(self, fid_list, fid_token_list, to_pdir_fid, pwd_id, stoken):
        return {"code": 0, "data": {"task_id": "task-%d" % len(fid_list)}}

    def query_task(self, task_id):
        count = int(task_id.split("-")[1])
        ids = []
        for _ in range(count):
            self.top += 1
            ids.append("top-%04d" % self.top)
        return {"code": 0, "data": {"save_as": {"save_as_top_fids": ids}}}

    def rename(self, fid, file_name):
        self.renames.append({"fid": fid, "name": file_name})
        return {"code": 0}

    def delete(self, fids):
        return {"code": 0, "data": {"task_id": "task-0"}}

    def recycle_list(self, page=1, size=30):
        return []

    def recycle_remove(self, record_list):
        return {"code": 0}


def run(fixture_path, out_path):
    with open(fixture_path, encoding="utf-8") as f:
        fixture = json.load(f)
    qas.CONFIG_DATA = {"magic_regex": fixture.get("magic_regex", {})}

    plan = []
    for raw_task in fixture["tasks"]:
        task = copy.deepcopy(raw_task)
        account = FakeQuark(fixture["share"], fixture["dest"])
        sink = contextlib.redirect_stdout(sys.stderr)
        with sink:
            tree = account.do_save_task(task)
        saved = []

        def collect(node):
            for child in tree.children(node.identifier):
                data = child.data
                if data.get("file_name"):
                    saved.append(
                        {
                            "path": data.get("path", ""),
                            "file_name": data["file_name"],
                            "file_name_re": data.get("file_name_re", ""),
                            "dir": bool(data.get("is_dir")),
                        }
                    )
                collect(child)

        if tree:
            collect(tree.get_node(tree.root))
        plan.append(
            {
                "taskname": task["taskname"],
                "saved": saved,
                "renames": account.renames,
                "ban": task.get("shareurl_ban", ""),
            }
        )
        print(
            "%-24s saved=%d renamed=%d ban=%s"
            % (task["taskname"][:24], len(saved), len(account.renames), task.get("shareurl_ban", "")),
            file=sys.stderr,
        )

    payload = {"source": "python quark_auto_save.py", "tasks": plan}
    with open(out_path, "w", encoding="utf-8") as f:
        json.dump(payload, f, ensure_ascii=False, indent=2)
        f.write("\n")
    print("wrote %s" % out_path, file=sys.stderr)


if __name__ == "__main__":
    fixture_path = sys.argv[1] if len(sys.argv) > 1 else os.path.join(ROOT, "poc", "quark", "testdata", "real_tasks.json")
    out_path = sys.argv[2] if len(sys.argv) > 2 else os.path.join(ROOT, "poc", "quark", "testdata", "real_tasks.plan.json")
    run(fixture_path, out_path)
