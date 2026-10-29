import { useEffect, useRef, useState } from "react";
import {
  ChevronDown,
  FlaskConical,
  FolderOpen,
  Link2,
  Loader2,
  Play,
  Plus,
  Search,
  Sparkles,
  Trash2,
  TriangleAlert,
} from "lucide-react";
import type { AppConfig, PluginConfig, SearchResult, Task } from "../lib/types";
import { pluginLabels, weekLabels } from "../lib/mock";
import { previewRename, searchShares, validateShare } from "../lib/api";
import { pickSharedDirectory } from "../lib/trim";
import { cn } from "../lib/utils";
import { Badge } from "../components/ui/badge";
import { Button } from "../components/ui/button";
import { Field, SwitchRow } from "../components/ui/field";
import { Input, Textarea } from "../components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "../components/ui/select";
import {
  Sheet,
  SheetBody,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from "../components/ui/sheet";
import { TaskStatusBadge } from "../components/TaskStatusBadge";
import { FilePickerDialog } from "./FilePickerDialog";
import { LocalDirDialog } from "./LocalDirDialog";
import { SearchPreviewDialog } from "./SearchPreviewDialog";

const emptyTask: Task = {
  taskname: "",
  shareurl: "",
  savepath: "",
  pattern: "",
  replace: "",
  runweek: [1, 2, 3, 4, 5, 6, 7],
  addition: {},
  latest: "—",
  next_run: "—",
  status: "waiting",
  progress: { saved: 0, expected: 0, days: [0, 0, 0, 0, 0, 0, 0] },
};

const pluginFieldLabels: Record<string, string> = {
  try_match: "按任务名匹配媒体库",
  media_id: "媒体库 ID",
  auto_refresh: "自动刷新媒体库",
  mdb_name: "媒体库名称",
  mdb_dir_list: "目录列表（逗号分隔）",
  mdb_guid: "媒体库 GUID",
  dir_list: "刷新目录",
  auto_gen: "自动生成 STRM",
  enable: "启用",
  auto_clean: "解压后删除压缩包",
  auto_clean_zipdir: "删除占位目录",
  auto_download: "自动加入下载",
  download_subdir: "递归子目录",
  save_path: "下载子目录",
  pause: "添加后暂停",
  save_path_mode: "保存路径模式",
  verify_path: "验证目录",
  full_path_mode: "完整路径模式",
};

const pluginEnabledKeys: Record<string, string> = {
  auto_unarchive: "enable",
  alist_strm_gen: "auto_gen",
  alist_sync: "enable",
  aria2: "auto_download",
};

function pluginEnabled(
  plugin: string,
  values: Record<string, string | number | boolean>,
) {
  const key = pluginEnabledKeys[plugin];
  if (key && key in values) return values[key] === true;
  return Object.values(values).some((value) => value === true);
}

function pluginFields(
  values: Record<string, string | number | boolean>,
  onChange: (key: string, value: string | number | boolean) => void,
) {
  return Object.entries(values).map(([key, value]) => {
    if (typeof value === "boolean") {
      return (
        <SwitchRow
          key={key}
          label={pluginFieldLabels[key] ?? key}
          checked={value}
          onCheckedChange={(checked) => onChange(key, checked)}
        />
      );
    }
    return (
      <Field key={key} label={pluginFieldLabels[key] ?? key} className="py-1.5">
        <Input
          value={String(value)}
          inputMode={typeof value === "number" ? "numeric" : undefined}
          onChange={(event) =>
            onChange(
              key,
              typeof value === "number"
                ? Number(event.target.value)
                : event.target.value,
            )
          }
        />
      </Field>
    );
  });
}

type TaskSheetProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  task: Task | null;
  magicRegex: AppConfig["magic_regex"];
  pluginDefaults: Record<string, PluginConfig>;
  searchReady: boolean;
  onOpenSettings: () => void;
  onSave: (task: Task, runAfter: boolean) => void;
  onTry: (task: Task) => void;
  onDelete: (task: Task) => void;
  onNotify: (message: string, tone?: "info" | "ok" | "warn") => void;
};

export function TaskSheet({
  open,
  onOpenChange,
  task,
  magicRegex,
  pluginDefaults,
  searchReady,
  onOpenSettings,
  onSave,
  onTry,
  onDelete,
  onNotify,
}: TaskSheetProps) {
  const [draft, setDraft] = useState<Task>(emptyTask);
  const [showAdvanced, setShowAdvanced] = useState(false);
  const [showSearch, setShowSearch] = useState(false);
  const [query, setQuery] = useState("");
  const [hits, setHits] = useState<SearchResult[]>([]);
  const [searchMessages, setSearchMessages] = useState<string[]>([]);
  const [searching, setSearching] = useState(false);
  const [previewHit, setPreviewHit] = useState<SearchResult | null>(null);
  const searchSeq = useRef(0);
  const [linkState, setLinkState] = useState<"unknown" | "ok" | "bad">("unknown");
  const [validating, setValidating] = useState(false);
  const [picker, setPicker] = useState<null | "savepath" | "share">(null);
  const [openPlugin, setOpenPlugin] = useState<string | null>(null);
  const [showRaw, setShowRaw] = useState(false);
  const [pickingLocal, setPickingLocal] = useState(false);
  const [localPickerOpen, setLocalPickerOpen] = useState(false);

  useEffect(() => {
    if (!open) return;
    const next = task
      ? structuredClone(task)
      : {
          ...structuredClone(emptyTask),
          pattern: Object.keys(magicRegex)[0] ?? "",
        };
    setDraft(next);
    setShowAdvanced(false);
    setShowSearch(false);
    setQuery("");
    setHits([]);
    setSearchMessages([]);
    setPreviewHit(null);
    setShowRaw(false);
    setOpenPlugin(Object.keys(next.addition)[0] ?? null);
    setLinkState(task?.shareurl_ban ? "bad" : next.shareurl ? "ok" : "unknown");
  }, [open, task, magicRegex]);

  useEffect(() => {
    if (!showSearch) return;
    const seq = (searchSeq.current += 1);
    const keyword = query.trim();
    if (!searchReady || !keyword) {
      setHits([]);
      setSearchMessages([]);
      setSearching(false);
      return;
    }
    const timer = window.setTimeout(() => {
      setSearching(true);
      searchShares(keyword)
        .then((result) => {
          if (searchSeq.current !== seq) return;
          setHits(result.results);
          setSearchMessages(result.messages);
        })
        .catch((cause: unknown) => {
          if (searchSeq.current !== seq) return;
          setHits([]);
          setSearchMessages([cause instanceof Error ? cause.message : "搜索失败"]);
        })
        .finally(() => {
          if (searchSeq.current === seq) setSearching(false);
        });
    }, 350);
    return () => window.clearTimeout(timer);
  }, [query, showSearch, searchReady]);

  const isNew = !task;
  const banned = Boolean(draft.shareurl_ban);
  const [samplePreview, setSamplePreview] = useState("");

  useEffect(() => {
    if (!open) return;
    if (!draft.pattern) {
      setSamplePreview("不筛选，保持原名");
      return;
    }
    let cancelled = false;
    const timer = window.setTimeout(() => {
      previewRename({
        taskname: draft.taskname,
        pattern: draft.pattern,
        replace: draft.replace,
        files: ["Black.Mirror.S07E05.mkv"],
      })
        .then((result) => {
          if (cancelled) return;
          const file = result.files[0];
          setSamplePreview(file?.matched ? file.renamed : "样本不匹配，跳过");
        })
        .catch(() => {
          if (!cancelled) setSamplePreview("预览失败");
        });
    }, 250);
    return () => {
      cancelled = true;
      window.clearTimeout(timer);
    };
  }, [open, draft.pattern, draft.replace, draft.taskname]);

  const update = <K extends keyof Task>(key: K, value: Task[K]) =>
    setDraft((prev) => ({ ...prev, [key]: value }));

  const pickLocalDirectory = async () => {
    setPickingLocal(true);
    try {
      const dir = await pickSharedDirectory();
      if (!dir) return;
      update("local_download", {
        ...(draft.local_download ?? { enable: true }),
        enable: true,
        dir,
      });
    } catch (cause) {
      onNotify(
        cause instanceof Error ? cause.message : "无法打开飞牛目录选择器",
        "warn",
      );
    } finally {
      setPickingLocal(false);
    }
  };

  // Overrides equal to the plugin default are dropped, so flipping a switch
  // back leaves no stale override behind.
  const updateAddition = (
    plugin: string,
    key: string,
    value: string | number | boolean,
  ) =>
    setDraft((prev) => {
      const defaults = pluginDefaults[plugin] ?? {};
      const merged: Record<string, string | number | boolean> = {
        ...(prev.addition[plugin] ?? {}),
        [key]: value,
      };
      for (const field of Object.keys(merged)) {
        if (field in defaults && String(merged[field]) === String(defaults[field])) {
          delete merged[field];
        }
      }
      const addition = { ...prev.addition };
      if (Object.keys(merged).length === 0) {
        delete addition[plugin];
      } else {
        addition[plugin] = merged;
      }
      return { ...prev, addition };
    });

  const pluginKeys = Array.from(
    new Set([...Object.keys(pluginDefaults), ...Object.keys(draft.addition)]),
  );

  const validate = async () => {
    if (!draft.shareurl.trim()) {
      onNotify("请先填写分享链接", "warn");
      return;
    }
    setValidating(true);
    try {
      const result = await validateShare(draft.shareurl);
      if (result.ok) {
        setLinkState("ok");
        onNotify(
          `链接有效：${result.title || "未命名分享"} · ${result.count ?? 0} 项`,
          "ok",
        );
      } else {
        setLinkState("bad");
        onNotify(result.message || "链接无效或已失效", "warn");
      }
    } catch (cause) {
      setLinkState("bad");
      onNotify(cause instanceof Error ? cause.message : "验证失败", "warn");
    } finally {
      setValidating(false);
    }
  };

  const submit = (runAfter: boolean) => {
    if (!draft.taskname.trim() || !draft.shareurl.trim() || !draft.savepath.trim()) {
      onNotify("任务名称、分享链接、保存路径为必填项", "warn");
      return;
    }
    const next: Task = {
      ...draft,
      shareurl_ban: undefined,
      status: banned ? "tracking" : draft.status,
    };
    onSave(next, runAfter);
    onOpenChange(false);
  };

  const tryRun = () => {
    if (!draft.taskname.trim() || !draft.shareurl.trim() || !draft.savepath.trim()) {
      onNotify("任务名称、分享链接、保存路径为必填项", "warn");
      return;
    }
    onTry({
      ...draft,
      shareurl_ban: undefined,
      status: banned ? "tracking" : draft.status,
    });
    onNotify("试运行已开始，可点「运行记录」查看日志", "ok");
  };

  const searchPanel = showSearch ? (
    <div className="rounded-md border border-line bg-line-soft/40 p-3">
      {!searchReady ? (
        <div className="flex items-start gap-2.5">
          <TriangleAlert className="mt-0.5 h-4 w-4 shrink-0 text-warn" />
          <div className="min-w-0 flex-1">
            <div className="text-[12px] font-medium text-ink">
              还没有可用的搜索源
            </div>
            <div className="mt-0.5 text-[12px] text-muted">
              先到设置里启用 PanSou 或 CloudSaver；装在本机时可以直接点「自动识别」。
            </div>
            <div className="mt-2.5 flex items-center gap-2">
              <Button size="sm" onClick={onOpenSettings}>
                去设置搜索源
              </Button>
              <Button
                variant="ghost"
                size="sm"
                onClick={() => setShowSearch(false)}
              >
                收起
              </Button>
            </div>
          </div>
        </div>
      ) : (
        <>
          <div className="flex items-center gap-2">
            <Search className="h-3.5 w-3.5 text-subtle" />
            <Input
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              placeholder="搜索资源，例如：黑镜"
              className="h-7"
            />
            <Button
              variant="ghost"
              size="sm"
              onClick={() => setShowSearch(false)}
            >
              收起
            </Button>
          </div>
          <div className="mt-2 space-y-1.5">
            {searching ? (
              <div className="flex items-center gap-2 px-1 py-2 text-[12px] text-subtle">
                <Loader2 className="h-3.5 w-3.5 animate-spin" />
                正在搜索…
              </div>
            ) : null}
            {!searching && searchMessages.length > 0 ? (
              <div className="rounded border border-warn/30 bg-warn-soft px-2.5 py-1.5 text-[12px] text-warn">
                {searchMessages.join("；")}
              </div>
            ) : null}
            {!searching &&
            query.trim() &&
            hits.length === 0 &&
            searchMessages.length === 0 ? (
              <div className="px-1 py-2 text-[12px] text-subtle">
                没有找到相关分享。
              </div>
            ) : null}
            {!searching && hits.length > 0 ? (
              <div className="px-1 text-[12px] text-subtle">
                点击候选可预览分享文件，再决定是否使用。
              </div>
            ) : null}
            {hits.map((hit) => (
              <button
                key={hit.shareurl}
                type="button"
                title="点击预览分享文件"
                className="flex w-full items-start gap-2 rounded border border-transparent bg-panel px-2.5 py-2 text-left transition-colors hover:border-brand/30 hover:bg-brand-soft/40"
                onClick={() => setPreviewHit(hit)}
              >
                <span className="mt-1 h-1.5 w-1.5 shrink-0 rounded-full bg-ok" />
                <span className="min-w-0 flex-1">
                  <span className="block truncate text-[12px] font-medium text-ink">
                    {hit.taskname}
                  </span>
                  <span className="mt-0.5 block truncate text-[12px] text-muted">
                    {hit.content}
                  </span>
                </span>
                <span className="shrink-0 font-mono text-[12px] text-subtle">
                  {hit.channel || hit.source}
                </span>
              </button>
            ))}
          </div>
        </>
      )}
    </div>
  ) : null;

  return (
    <>
      <Sheet open={open} onOpenChange={onOpenChange}>
        <SheetContent>
          <SheetHeader>
            <div className="flex items-center gap-2">
              <SheetTitle>{isNew ? "新增任务" : draft.taskname}</SheetTitle>
              {isNew ? null : <TaskStatusBadge status={draft.status} />}
            </div>
            <SheetDescription>
              {isNew
                ? "填写分享链接与保存规则；保存后可立即运行。"
                : "编辑任务配置，或修复失效链接后立即重跑。"}
            </SheetDescription>
          </SheetHeader>

          <SheetBody className="space-y-4">
            {banned ? (
              <div className="flex items-start gap-2 rounded-md border border-danger/20 bg-danger-soft px-3 py-2.5 text-danger">
                <TriangleAlert className="mt-0.5 h-4 w-4 shrink-0" />
                <div className="min-w-0 flex-1 text-[12px] leading-5">
                  <b className="font-semibold">该分享链接已失效</b>
                  <div className="opacity-80">{draft.shareurl_ban}</div>
                </div>
                <Button
                  variant="danger"
                  size="sm"
                  onClick={() => setShowSearch(true)}
                >
                  查找替换
                </Button>
              </div>
            ) : null}

            <section className="space-y-3">
              <div className="flex items-center justify-between">
                <h3 className="text-[12px] font-semibold uppercase tracking-[0.08em] text-muted">
                  基本信息
                </h3>
                <Badge variant={linkState === "bad" ? "danger" : linkState === "ok" ? "ok" : "neutral"}>
                  {linkState === "bad" ? "链接异常" : linkState === "ok" ? "链接有效" : "未验证"}
                </Badge>
              </div>
              <Field label="任务名称" htmlFor="taskname">
                <Input
                  id="taskname"
                  value={draft.taskname}
                  onChange={(event) => update("taskname", event.target.value)}
                  placeholder="例如：黑镜"
                />
              </Field>
              <Field
                label="分享链接"
                htmlFor="shareurl"
                hint="支持子目录链接与提取码链接。链接失效时，任务会被标记并跳过转存。"
              >
                <div className="flex gap-1.5">
                  <Input
                    id="shareurl"
                    value={draft.shareurl}
                    onChange={(event) => {
                      update("shareurl", event.target.value);
                      setLinkState("unknown");
                    }}
                    placeholder="https://pan.quark.cn/s/..."
                  />
                  <Button
                    variant="secondary"
                    size="icon"
                    aria-label="验证链接"
                    disabled={validating}
                    onClick={() => void validate()}
                  >
                    {validating ? (
                      <Loader2 className="h-3.5 w-3.5 animate-spin" />
                    ) : (
                      <Link2 className="h-3.5 w-3.5" />
                    )}
                  </Button>
                  <Button variant="secondary" size="sm" onClick={() => setShowSearch(true)}>
                    <Search className="h-3.5 w-3.5" />
                    搜索
                  </Button>
                </div>
              </Field>
              {searchPanel}
              <Field label="保存路径" htmlFor="savepath">
                <div className="flex gap-1.5">
                  <Input
                    id="savepath"
                    value={draft.savepath}
                    onChange={(event) => update("savepath", event.target.value)}
                    placeholder="/video/tv/..."
                  />
                  <Button
                    variant="secondary"
                    size="sm"
                    onClick={() => setPicker("savepath")}
                  >
                    <FolderOpen className="h-3.5 w-3.5" />
                    选择
                  </Button>
                </div>
              </Field>
              <SwitchRow
                label="下载到 NAS"
                description="转存完成后，把本次新增文件拉取到本地目录。"
                checked={Boolean(draft.local_download?.enable)}
                onCheckedChange={(checked) =>
                  update("local_download", {
                    ...(draft.local_download ?? {}),
                    enable: checked,
                  })
                }
              />
              {draft.local_download?.enable ? (
                <Field label="下载目录" hint="留空使用设置里的默认目录。">
                  <div className="flex gap-1.5">
                    <Input
                      value={draft.local_download?.dir ?? ""}
                      onChange={(event) =>
                        update("local_download", {
                          enable: true,
                          ...(draft.local_download ?? {}),
                          dir: event.target.value,
                        })
                      }
                      placeholder="/vol2/1000/影视"
                    />
                    <Button
                      variant="secondary"
                      size="sm"
                      onClick={() => setLocalPickerOpen(true)}
                    >
                      <FolderOpen className="h-3.5 w-3.5" />
                      选择目录
                    </Button>
                    <Button
                      variant="ghost"
                      size="sm"
                      disabled={pickingLocal}
                      onClick={() => void pickLocalDirectory()}
                    >
                      {pickingLocal ? (
                        <Loader2 className="h-3.5 w-3.5 animate-spin" />
                      ) : (
                        <Plus className="h-3.5 w-3.5" />
                      )}
                      新增授权
                    </Button>
                  </div>
                </Field>
              ) : null}
            </section>

            <section className="space-y-3 border-t border-line pt-3.5">
              <div className="flex items-center justify-between">
                <h3 className="text-[12px] font-semibold uppercase tracking-[0.08em] text-muted">
                  保存规则
                </h3>
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => setPicker("share")}
                >
                  <Sparkles className="h-3.5 w-3.5" />
                  正则预览
                </Button>
              </div>
              <div className="grid gap-2 sm:grid-cols-[1.1fr_1fr]">
                <Field label="匹配表达式">
                  <Select
                    value={draft.pattern || "__all__"}
                    onValueChange={(value) =>
                      update("pattern", value === "__all__" ? "" : value)
                    }
                  >
                    <SelectTrigger aria-label="匹配表达式">
                      <SelectValue placeholder="自定义 / 留空转存全部" />
                    </SelectTrigger>
                    <SelectContent>
                      {Object.entries(magicRegex).map(([key, value]) => (
                        <SelectItem key={key} value={key}>
                          {key} · {value.pattern.slice(0, 28)}
                        </SelectItem>
                      ))}
                      <SelectItem value="__all__">自定义 / 留空转存全部</SelectItem>
                    </SelectContent>
                  </Select>
                </Field>
                <Field label="替换表达式">
                  <Input
                    value={draft.replace}
                    onChange={(event) => update("replace", event.target.value)}
                    placeholder="留空使用魔法变量"
                  />
                </Field>
              </div>
              <div className="rounded-md border border-line bg-line-soft/40 px-2.5 py-2">
                <div className="font-mono text-[12px] uppercase tracking-[0.08em] text-subtle">
                  PREVIEW
                </div>
                <div className="mt-1 flex items-center gap-2 overflow-hidden font-mono text-[12px]">
                  <span className="truncate text-muted">Black.Mirror.S07E05.mkv</span>
                  <span className="text-subtle">→</span>
                  <span className="truncate font-medium text-ink">
                    {samplePreview || "计算中…"}
                  </span>
                </div>
              </div>
              <SwitchRow
                label="忽略文件后缀"
                description="01.mp4 与 01.mkv 视为同一文件，避免重复转存。"
                checked={Boolean(draft.ignore_extension)}
                onCheckedChange={(checked) => update("ignore_extension", checked)}
              />
            </section>

            <section className="space-y-3 border-t border-line pt-3.5">
              <h3 className="text-[12px] font-semibold uppercase tracking-[0.08em] text-muted">
                排程
              </h3>
              <div className="grid gap-3 sm:grid-cols-[1fr_1.1fr]">
                <Field label="截止日期" hint="留空表示永久运行">
                  <Input
                    type="date"
                    value={draft.enddate ?? ""}
                    onChange={(event) => update("enddate", event.target.value)}
                  />
                </Field>
                <Field label="运行星期">
                  <div className="grid grid-cols-7 gap-1">
                    {weekLabels.map((label, index) => {
                      const day = index + 1;
                      const checked = draft.runweek.includes(day);
                      return (
                        <button
                          key={day}
                          type="button"
                          aria-pressed={checked}
                          aria-label={`周${label}`}
                          onClick={() =>
                            update(
                              "runweek",
                              checked
                                ? draft.runweek.filter((item) => item !== day)
                                : [...draft.runweek, day].sort(),
                            )
                          }
                          className={cn(
                            "flex h-7 items-center justify-center rounded border text-[12px] transition-colors",
                            checked
                              ? "border-brand bg-brand font-medium text-white"
                              : "border-line bg-panel text-muted hover:border-brand/40 hover:bg-brand-soft hover:text-brand-ink",
                          )}
                        >
                          {label}
                        </button>
                      );
                    })}
                  </div>
                </Field>
              </div>
              <button
                type="button"
                onClick={() => setShowAdvanced((prev) => !prev)}
                className="flex w-full items-center justify-between rounded-md border border-line bg-panel px-2.5 py-2 text-[12px] text-muted hover:bg-line-soft"
              >
                <span>高级：起始文件、子目录更新</span>
                <ChevronDown
                  className={cn("h-3.5 w-3.5 transition-transform", showAdvanced && "rotate-180")}
                />
              </button>
              {showAdvanced ? (
                <div className="space-y-3 rounded-md border border-line bg-line-soft/30 p-3">
                  <Field label="文件起始 fid" hint="只转存修改日期晚于该文件的文件">
                    <Input
                      value={draft.startfid ?? ""}
                      onChange={(event) => update("startfid", event.target.value)}
                      placeholder="可选"
                    />
                  </Field>
                  <Field label="更新子目录" hint="匹配子目录名的正则，多项用 | 分隔">
                    <Input
                      value={draft.update_subdir ?? ""}
                      onChange={(event) => update("update_subdir", event.target.value)}
                      placeholder="4k|1080p"
                    />
                  </Field>
                  <SwitchRow
                    label="重存模式"
                    description="删除该子目录后重新转存，适合大资源包；关闭时递归更新。"
                    checked={Boolean(draft.update_subdir_resave_mode)}
                    onCheckedChange={(checked) =>
                      update("update_subdir_resave_mode", checked)
                    }
                  />
                </div>
              ) : null}
            </section>

            <section className="space-y-2 border-t border-line pt-3.5">
              <div className="flex items-center justify-between">
                <h3 className="text-[12px] font-semibold uppercase tracking-[0.08em] text-muted">
                  插件选项
                </h3>
              </div>
              {pluginKeys.map((plugin) => {
                const values = {
                  ...(pluginDefaults[plugin] ?? {}),
                  ...(draft.addition[plugin] ?? {}),
                };
                if (Object.keys(values).length === 0) return null;
                const enabled = pluginEnabled(plugin, values);
                const isOpen = openPlugin === plugin;
                return (
                  <div
                    key={plugin}
                    className="overflow-hidden rounded-md border border-line bg-panel"
                  >
                    <button
                      type="button"
                      aria-expanded={isOpen}
                      onClick={() => setOpenPlugin(isOpen ? null : plugin)}
                      className="flex w-full items-center gap-2 px-3 py-2 text-left transition-colors hover:bg-line-soft/40"
                    >
                      <span className="min-w-0 flex-1 truncate text-[12px] font-medium text-ink">
                        {pluginLabels[plugin] ?? plugin}
                      </span>
                      <span className="hidden font-mono text-[12px] uppercase tracking-[0.08em] text-subtle sm:inline">
                        {plugin}
                      </span>
                      <Badge variant={enabled ? "ok" : "neutral"} dot>
                        {enabled ? "已启用" : "未启用"}
                      </Badge>
                      <ChevronDown
                        className={cn(
                          "h-3.5 w-3.5 shrink-0 text-subtle transition-transform",
                          isOpen && "rotate-180",
                        )}
                      />
                    </button>
                    {isOpen ? (
                      <div className="border-t border-line-soft px-3 py-2">
                        {pluginFields(values, (key, value) =>
                          updateAddition(plugin, key, value),
                        )}
                      </div>
                    ) : null}
                  </div>
                );
              })}
            </section>

            <section className="border-t border-line pt-3.5">
              <button
                type="button"
                aria-expanded={showRaw}
                onClick={() => setShowRaw((prev) => !prev)}
                className="flex w-full items-center justify-between text-[12px] text-muted transition-colors hover:text-ink"
              >
                原始配置 JSON
                <ChevronDown
                  className={cn(
                    "h-3.5 w-3.5 text-subtle transition-transform",
                    showRaw && "rotate-180",
                  )}
                />
              </button>
              {showRaw ? (
                <Textarea
                  value={JSON.stringify(draft.addition, null, 2)}
                  readOnly
                  className="mt-2 h-24 font-mono text-[12px] text-muted"
                  aria-label="任务插件配置 JSON"
                />
              ) : null}
            </section>
          </SheetBody>

          <SheetFooter className="justify-between">
            {isNew ? (
              <span className="text-[12px] text-subtle">
                保存后可在任务列表继续编辑
              </span>
            ) : (
              <Button
                variant="ghost"
                size="sm"
                className="text-danger hover:bg-danger-soft hover:text-danger"
                onClick={() => {
                  onDelete(draft);
                  onOpenChange(false);
                }}
              >
                <Trash2 className="h-3.5 w-3.5" />
                删除任务
              </Button>
            )}
            <div className="flex items-center gap-2">
              <Button variant="secondary" size="sm" onClick={() => onOpenChange(false)}>
                取消
              </Button>
              <Button variant="secondary" size="sm" onClick={tryRun}>
                <FlaskConical className="h-3.5 w-3.5" />
                试运行
              </Button>
              <Button variant="secondary" size="sm" onClick={() => submit(false)}>
                保存
              </Button>
              <Button size="sm" onClick={() => submit(true)}>
                {banned ? <Loader2 className="h-3.5 w-3.5" /> : <Play className="h-3.5 w-3.5" />}
                保存并运行
              </Button>
            </div>
          </SheetFooter>
        </SheetContent>
      </Sheet>

      <FilePickerDialog
        open={picker !== null}
        mode={picker ?? "savepath"}
        task={draft}
        onOpenChange={(next) => {
          if (!next) setPicker(null);
        }}
        onPickPath={(path) => {
          update("savepath", path);
          setPicker(null);
          onNotify(`保存路径已设为 ${path}`, "ok");
        }}
        onNotify={onNotify}
      />

      <LocalDirDialog
        open={localPickerOpen}
        initial={draft.local_download?.dir}
        onOpenChange={setLocalPickerOpen}
        onSelect={(path) => {
          update("local_download", {
            enable: true,
            ...(draft.local_download ?? {}),
            dir: path,
          });
          onNotify(`下载目录已设为 ${path}`, "ok");
        }}
        onNotify={onNotify}
      />

      <SearchPreviewDialog
        open={previewHit !== null}
        hit={previewHit}
        task={draft}
        onOpenChange={(next) => {
          if (!next) setPreviewHit(null);
        }}
        onUse={(hit) => {
          setDraft((prev) => ({
            ...prev,
            shareurl: hit.shareurl,
            shareurl_ban: undefined,
            status: "tracking",
          }));
          setLinkState("ok");
          setPreviewHit(null);
          setShowSearch(false);
          onNotify("已填入候选链接，保存后生效", "ok");
        }}
      />
    </>
  );
}
