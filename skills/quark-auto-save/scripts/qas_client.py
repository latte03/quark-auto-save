#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""QAS 控制台 HTTP 客户端（stdlib，无第三方依赖）。

鉴权：优先 QAS_TOKEN（服务端 API token，作为 ?token= 传递）；
没有 token 时可用 QAS_USER + QAS_PASSWORD 登录，会话 cookie 只留在本次进程内。

输出协议：成功打印 `OK` 或 `OK <单行 JSON>`，失败打印 `ERROR: 原因`。
"""

import argparse
import http.cookiejar
import io
import json
import os
import sys
import urllib.error
import urllib.parse
import urllib.request

sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding="utf-8", errors="replace")
sys.stderr = io.TextIOWrapper(sys.stderr.buffer, encoding="utf-8", errors="replace")

BASE_URL = os.environ.get("QAS_BASE_URL", "").rstrip("/")
TOKEN = os.environ.get("QAS_TOKEN", "").strip()
USER = os.environ.get("QAS_USER", "").strip()
PASSWORD = os.environ.get("QAS_PASSWORD", "")
TIMEOUT = int(os.environ.get("QAS_TIMEOUT", "60"))

# Keys the task editor computes; sending them back would persist junk.
DERIVED_TASK_KEYS = ("index", "status", "latest", "next_run", "progress", "magic_name")


class ApiError(Exception):
    pass


class Client:
    def __init__(self) -> None:
        if not BASE_URL:
            raise ApiError("QAS_BASE_URL 未设置（例如 http://192.168.1.10:8710）")
        self.opener = urllib.request.build_opener(
            urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar())
        )
        self.token = TOKEN
        if not self.token and USER and PASSWORD:
            self.call("POST", "/api/login", {"username": USER, "password": PASSWORD}, auth=False)

    def call(self, method, path, body=None, query=None, auth=True, raw=False):
        url = BASE_URL + path
        params = dict(query or {})
        if auth and self.token:
            params["token"] = self.token
        if params:
            url += "?" + urllib.parse.urlencode(params)
        data = json.dumps(body).encode("utf-8") if body is not None else None
        request = urllib.request.Request(
            url,
            data=data,
            method=method,
            headers={"Content-Type": "application/json"} if data else {},
        )
        try:
            response = self.opener.open(request, timeout=TIMEOUT)
        except urllib.error.HTTPError as exc:
            text = exc.read().decode("utf-8", "replace")
            raise ApiError(f"HTTP {exc.code}: {_message(text) or exc.reason}") from exc
        except urllib.error.URLError as exc:
            raise ApiError(f"无法连接 {BASE_URL}: {exc.reason}") from exc
        text = response.read().decode("utf-8", "replace")
        if raw:
            return text
        try:
            payload = json.loads(text)
        except json.JSONDecodeError:
            return text
        if isinstance(payload, dict) and payload.get("success") is False:
            raise ApiError(payload.get("message") or "请求失败")
        if isinstance(payload, dict) and "data" in payload:
            return payload["data"]
        return payload

    # ---------- helpers ----------

    def tasks(self):
        return self.call("GET", "/api/tasks")["tasks"]

    def find_task(self, name):
        exact = [task for task in self.tasks() if task.get("taskname") == name]
        if not exact:
            loose = [task for task in self.tasks() if name in (task.get("taskname") or "")]
            if len(loose) > 1:
                names = "、".join(task["taskname"] for task in loose)
                raise ApiError(f"「{name}」匹配到多个任务：{names}")
            exact = loose
        if not exact:
            raise ApiError(f"任务「{name}」不存在")
        return exact[0]

    def put_task(self, index, task):
        for key in DERIVED_TASK_KEYS:
            task.pop(key, None)
        return self.call("PUT", f"/api/tasks/{index}", {"task": task})["tasks"]

    def watch(self, run_id, emit=print):
        """打印 SSE 日志直到 [DONE]，返回是否正常结束。"""
        url = f"{BASE_URL}/api/runs/{urllib.parse.quote(run_id)}/stream"
        if self.token:
            url += "?" + urllib.parse.urlencode({"token": self.token})
        request = urllib.request.Request(url, headers={"Accept": "text/event-stream"})
        # Log lines arrive continuously; give a long run room between them.
        with self.opener.open(request, timeout=max(TIMEOUT, 300)) as response:
            for raw in response:
                line = raw.decode("utf-8", "replace").rstrip("\r\n")
                if not line.startswith("data:"):
                    continue
                content = line[5:].lstrip()
                if content == "[DONE]":
                    return True
                if content:
                    emit(content)
        return False


def _message(text):
    try:
        payload = json.loads(text)
    except json.JSONDecodeError:
        return text.strip()[:300]
    if isinstance(payload, dict):
        return payload.get("message") or ""
    return ""


def ok(data=None):
    if data is None:
        print("OK")
    else:
        print("OK " + json.dumps(data, separators=(",", ":"), ensure_ascii=False))


def read_json_arg(value):
    if value.startswith("@"):
        with open(value[1:], "r", encoding="utf-8") as handle:
            return json.load(handle)
    return json.loads(value)


# ---------- commands ----------


def cmd_status(client, _args):
    ok(client.call("GET", "/api/status"))


def cmd_config(client, args):
    config = client.call("GET", "/api/config")
    config.pop("cookie", None)
    config.pop("api_token", None)
    if not args.full:
        config.pop("plugins", None)
        config.pop("push_config", None)
    ok(config)


def cmd_config_set(client, args):
    partial = read_json_arg(args.payload)
    if not isinstance(partial, dict):
        raise ApiError("配置修改必须是 JSON 对象")
    if "cookie" in partial:
        raise ApiError("不要通过 CLI 改 cookie：请在控制台扫码登录，或直接编辑服务器上的 config.json")
    updated = client.call("PUT", "/api/config", partial)
    updated.pop("cookie", None)
    updated.pop("api_token", None)
    ok({"updated": sorted(partial)})


def cmd_magic(client, _args):
    config = client.call("GET", "/api/config")
    ok(config.get("magic_regex", []))


def cmd_tasks(client, args):
    tasks = client.tasks()
    if args.json:
        ok(tasks)
        return
    rows = [
        {
            "index": task.get("index"),
            "taskname": task.get("taskname"),
            "status": task.get("status"),
            "runtime": task.get("runtime", []),
            "runweek": task.get("runweek", []),
            "next_run": task.get("next_run"),
            "magic": task.get("magic_name") or task.get("pattern") or "",
            "savepath": task.get("savepath"),
        }
        for task in tasks
    ]
    ok(rows)


def cmd_task(client, args):
    ok(client.find_task(args.name))


def cmd_add(client, args):
    task = read_json_arg(args.payload)
    tasks = client.call("POST", "/api/tasks", {"task": task})["tasks"]
    created = next(
        (item for item in tasks if item.get("taskname") == task.get("taskname")), None
    )
    ok({"task": created, "total": len(tasks)})
    if args.run and created is not None:
        _start_and_watch(client, [created["index"]], args.write, args.wait)


def cmd_update(client, args):
    task = client.find_task(args.name)
    index = task["index"]
    partial = read_json_arg(args.payload)
    if not isinstance(partial, dict):
        raise ApiError("修改内容必须是 JSON 对象")
    merged = {key: value for key, value in task.items() if key not in DERIVED_TASK_KEYS}
    merged.update(partial)
    tasks = client.put_task(index, merged)
    ok(next((item for item in tasks if item.get("taskname") == merged.get("taskname")), None))


def cmd_rm(client, args):
    task = client.find_task(args.name)
    if not args.yes:
        raise ApiError(
            f"将删除任务「{task['taskname']}」（序号 {task['index']}），确认后加 --yes 重跑"
        )
    tasks = client.call("DELETE", f"/api/tasks/{task['index']}")["tasks"]
    ok({"deleted": task["taskname"], "total": len(tasks)})


def cmd_run(client, args):
    if args.payload:
        task = read_json_arg(args.payload)
        run_id = client.call("POST", "/api/run", {"task": task, "write": args.write})["run_id"]
    elif args.target:
        task = client.find_task(args.target)
        run_id = client.call(
            "POST", "/api/run", {"tasks": [task["index"]], "write": args.write}
        )["run_id"]
    elif args.all:
        run_id = client.call("POST", "/api/run", {"tasks": [], "write": args.write})["run_id"]
    else:
        raise ApiError("指定任务名、--all，或用 run-json 传临时任务")
    ok({"run_id": run_id, "write": bool(args.write)})
    if args.wait:
        client.watch(run_id)


def _start_and_watch(client, indices, write, wait):
    run_id = client.call("POST", "/api/run", {"tasks": indices, "write": write})["run_id"]
    ok({"run_id": run_id, "write": bool(write)})
    if wait:
        client.watch(run_id)


def cmd_share(client, args):
    payload = {"shareurl": args.url}
    if args.task:
        task = read_json_arg(args.task)
        for key in ("taskname", "magic_id", "pattern", "replace", "savepath", "update_subdir", "ignore_extension"):
            if key in task:
                payload[key] = task[key]
    data = client.call("POST", "/api/share/detail", payload)
    files = data.get("files", [])
    limit = len(files) if args.all else args.files
    shown = files[:limit]
    result = {
        "title": data.get("title"),
        "savepath": data.get("savepath"),
        "pattern": data.get("pattern"),
        "replace": data.get("replace"),
        "total": data.get("total"),
        "files": shown,
    }
    if len(files) > len(shown):
        result["note"] = f"还有 {len(files) - len(shown)} 个条目未显示，加 --all 查看"
    ok(result)


def cmd_validate(client, args):
    ok(client.call("POST", "/api/share/validate", {"shareurl": args.url}))


def cmd_browse(client, args):
    data = client.call("GET", "/api/browse", query={"path": args.path or "/"})
    entries = data.get("entries", [])
    dirs = [entry["name"] for entry in entries if entry.get("is_dir")]
    files = [entry["name"] for entry in entries if not entry.get("is_dir")]
    ok(
        {
            "path": data.get("path"),
            "dirs": dirs,
            "files": files[: args.files] if args.files else files,
            "total": len(entries),
        }
    )


def cmd_mkdir(client, args):
    ok(client.call("POST", "/api/files/mkdir", {"path": args.path}))


def cmd_rename(client, args):
    ok(client.call("POST", "/api/files/rename", {"path": args.path, "name": args.new_name}))


def cmd_rm_file(client, args):
    if not args.yes:
        raise ApiError(f"将把 {args.path} 移入网盘回收站，确认后加 --yes 重跑")
    ok(client.call("POST", "/api/files/delete", {"path": args.path, "confirm": True}))


def cmd_search(client, args):
    data = client.call(
        "GET", "/api/search", query={"keyword": args.keyword, "deep": "1" if args.deep else "0"}
    )
    results = data.get("results", [])
    ok(
        {
            "keyword": data.get("keyword"),
            "count": len(results),
            "messages": data.get("messages", []),
            "results": results[: args.limit],
        }
    )


def cmd_runs(client, args):
    runs = client.call("GET", "/api/runs")["runs"]
    ok(runs[: args.limit])


def cmd_log(client, args):
    data = client.call("GET", f"/api/runs/{urllib.parse.quote(args.run_id)}/log")
    lines = data.get("lines", [])
    if args.tail:
        lines = lines[-args.tail :]
    ok({"run": data.get("run"), "done": data.get("done")})
    for line in lines:
        print(line)


def cmd_watch(client, args):
    finished = client.watch(args.run_id)
    ok({"run_id": args.run_id, "finished": finished})


def cmd_sign(client, args):
    run_id = client.call("POST", "/api/sign", {})["run_id"]
    ok({"run_id": run_id})
    if args.wait:
        client.watch(run_id)


def cmd_notify_test(client, _args):
    ok(client.call("POST", "/api/notify/test", {}))


def cmd_downloads(client, _args):
    ok(client.call("GET", "/api/downloads"))


def cmd_session(client, _args):
    ok(client.call("GET", "/api/session", auth=False))


def build_parser():
    parser = argparse.ArgumentParser(
        description="QAS 控制台 HTTP 客户端",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog=(
            "环境变量：QAS_BASE_URL（必填）、QAS_TOKEN 或 QAS_USER+QAS_PASSWORD\n"
            "所有写操作默认演练：转存要加 --write；删除要加 --yes。"
        ),
    )
    sub = parser.add_subparsers(dest="command", required=True)

    sub.add_parser("status", help="任务数、失效数、当前运行").set_defaults(func=cmd_status)
    sub.add_parser("session", help="会话信息").set_defaults(func=cmd_session)

    config = sub.add_parser("config", help="读取配置（隐藏 cookie / token）")
    config.add_argument("--full", action="store_true", help="包含插件与推送渠道")
    config.set_defaults(func=cmd_config)

    config_set = sub.add_parser("config-set", help="按 JSON 局部更新配置")
    config_set.add_argument("payload", help="JSON 字符串或 @file")
    config_set.set_defaults(func=cmd_config_set)

    sub.add_parser("magic", help="列出魔法规则（id / 名称 / 正则）").set_defaults(func=cmd_magic)

    tasks = sub.add_parser("tasks", help="列出任务")
    tasks.add_argument("--json", action="store_true", help="输出完整任务字段")
    tasks.set_defaults(func=cmd_tasks)

    task = sub.add_parser("task", help="查看单个任务完整字段")
    task.add_argument("name")
    task.set_defaults(func=cmd_task)

    add = sub.add_parser("add", help="新增任务")
    add.add_argument("payload", help="任务 JSON 或 @file")
    add.add_argument("--run", action="store_true", help="创建后立即运行")
    add.add_argument("--write", action="store_true", help="真正转存（默认演练）")
    add.add_argument("--wait", action="store_true", help="跟随运行日志")
    add.set_defaults(func=cmd_add)

    update = sub.add_parser("update", help="按名称局部更新任务（键整体替换）")
    update.add_argument("name")
    update.add_argument("payload", help="要改的字段 JSON 或 @file")
    update.set_defaults(func=cmd_update)

    rm = sub.add_parser("rm", help="删除任务")
    rm.add_argument("name")
    rm.add_argument("--yes", action="store_true", help="确认删除")
    rm.set_defaults(func=cmd_rm)

    run = sub.add_parser("run", help="运行任务（默认演练）")
    run.add_argument("target", nargs="?", help="任务名；省略时配合 --all")
    run.add_argument("--all", action="store_true", help="运行全部未完成的定时任务")
    run.add_argument("--payload", help="临时任务 JSON 或 @file（不写入配置）")
    run.add_argument("--write", action="store_true", help="真正转存")
    run.add_argument("--wait", action="store_true", help="跟随运行日志")
    run.set_defaults(func=cmd_run)

    share = sub.add_parser("share", help="查看分享内容与命中/改名预览")
    share.add_argument("url")
    share.add_argument("--task", help="任务 JSON：提供 taskname / magic_id / pattern / replace / savepath")
    share.add_argument("--files", type=int, default=20, help="最多显示条目，默认 20")
    share.add_argument("--all", action="store_true", help="显示全部条目")
    share.set_defaults(func=cmd_share)

    validate = sub.add_parser("validate", help="校验分享链接是否可用")
    validate.add_argument("url")
    validate.set_defaults(func=cmd_validate)

    browse = sub.add_parser("browse", help="列出网盘目录")
    browse.add_argument("path", nargs="?", default="/")
    browse.add_argument("--files", type=int, default=30)
    browse.set_defaults(func=cmd_browse)

    mkdir = sub.add_parser("mkdir", help="新建网盘文件夹")
    mkdir.add_argument("path")
    mkdir.set_defaults(func=cmd_mkdir)

    rename = sub.add_parser("rename", help="重命名网盘文件")
    rename.add_argument("path")
    rename.add_argument("new_name")
    rename.set_defaults(func=cmd_rename)

    rm_file = sub.add_parser("rm-file", help="删除网盘文件（进回收站）")
    rm_file.add_argument("path")
    rm_file.add_argument("--yes", action="store_true")
    rm_file.set_defaults(func=cmd_rm_file)

    search = sub.add_parser("search", help="搜索分享（需配置搜索源）")
    search.add_argument("keyword")
    search.add_argument("--deep", action="store_true")
    search.add_argument("--limit", type=int, default=10)
    search.set_defaults(func=cmd_search)

    runs = sub.add_parser("runs", help="最近的运行记录")
    runs.add_argument("--limit", type=int, default=5)
    runs.set_defaults(func=cmd_runs)

    log = sub.add_parser("log", help="读取某次运行日志")
    log.add_argument("run_id")
    log.add_argument("--tail", type=int, default=0, help="只看最后 N 行")
    log.set_defaults(func=cmd_log)

    watch = sub.add_parser("watch", help="跟随运行中的日志（SSE）")
    watch.add_argument("run_id")
    watch.set_defaults(func=cmd_watch)

    sign = sub.add_parser("sign", help="全部账号签到")
    sign.add_argument("--wait", action="store_true")
    sign.set_defaults(func=cmd_sign)

    sub.add_parser("notify-test", help="发送一条测试推送").set_defaults(func=cmd_notify_test)
    sub.add_parser("downloads", help="本地下载队列").set_defaults(func=cmd_downloads)
    return parser


def main():
    args = build_parser().parse_args()
    try:
        client = Client()
        args.func(client, args)
    except ApiError as exc:
        print(f"ERROR: {exc}")
        sys.exit(1)
    except json.JSONDecodeError as exc:
        print(f"ERROR: JSON 解析失败: {exc}")
        sys.exit(1)
    except FileNotFoundError as exc:
        print(f"ERROR: 找不到文件: {exc.filename}")
        sys.exit(1)


if __name__ == "__main__":
    main()
