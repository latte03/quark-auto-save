import { useEffect, useState } from "react";
import {
  ChevronRight,
  FileArchive,
  FileVideo,
  Folder,
  FolderOpen,
  HardDrive,
  Home,
  Loader2,
  Sparkles,
} from "lucide-react";
import type { Task } from "../lib/types";
import { formatBytes } from "../lib/mock";
import { browse, shareDetail, type BrowseEntry, type ShareDetail } from "../lib/api";
import { cn } from "../lib/utils";
import { Badge } from "../components/ui/badge";
import { Button } from "../components/ui/button";
import {
  Dialog,
  DialogBody,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "../components/ui/dialog";
import { Input } from "../components/ui/input";
import { useToast } from "../components/ToastProvider";

type FilePickerDialogProps = {
  open: boolean;
  mode: "savepath" | "share";
  task: Task;
  onOpenChange: (open: boolean) => void;
  onPickPath: (path: string) => void;
};

export function FilePickerDialog({
  open,
  mode,
  task,
  onOpenChange,
  onPickPath,
}: FilePickerDialogProps) {
  const notify = useToast();
  const [path, setPath] = useState("/");
  const [manualPath, setManualPath] = useState("");
  const [entries, setEntries] = useState<BrowseEntry[]>([]);
  const [dirLoading, setDirLoading] = useState(false);
  const [dirError, setDirError] = useState("");

  const [preview, setPreview] = useState<ShareDetail | null>(null);
  const [previewLoading, setPreviewLoading] = useState(false);
  const [previewError, setPreviewError] = useState("");

  useEffect(() => {
    if (!open) return;
    const initial = task.savepath || "/";
    setPath(initial.startsWith("/") ? initial : `/${initial}`);
    setManualPath(initial);
  }, [open, task.savepath]);

  useEffect(() => {
    if (!open || mode !== "savepath") return;
    let cancelled = false;
    setDirLoading(true);
    setDirError("");
    browse(path)
      .then((result) => {
        if (!cancelled) {
          setEntries(result.entries.filter((entry) => entry.is_dir));
        }
      })
      .catch((cause: unknown) => {
        if (!cancelled) {
          setEntries([]);
          setDirError(cause instanceof Error ? cause.message : "读取目录失败");
        }
      })
      .finally(() => {
        if (!cancelled) setDirLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [open, mode, path]);

  useEffect(() => {
    if (!open || mode !== "share") return;
    if (!task.shareurl.trim()) {
      setPreview(null);
      setPreviewError("任务还没有分享链接");
      return;
    }
    let cancelled = false;
    setPreviewLoading(true);
    setPreviewError("");
    shareDetail({
      shareurl: task.shareurl,
      taskname: task.taskname,
      pattern: task.pattern,
      replace: task.replace,
      savepath: task.savepath,
      update_subdir: task.update_subdir,
      ignore_extension: task.ignore_extension,
    })
      .then((result) => {
        if (!cancelled) setPreview(result);
      })
      .catch((cause: unknown) => {
        if (!cancelled) {
          setPreview(null);
          setPreviewError(
            cause instanceof Error ? cause.message : "读取分享内容失败",
          );
        }
      })
      .finally(() => {
        if (!cancelled) setPreviewLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [
    open,
    mode,
    task.shareurl,
    task.taskname,
    task.pattern,
    task.replace,
    task.savepath,
    task.update_subdir,
    task.ignore_extension,
  ]);

  const breadcrumb = path.split("/").filter(Boolean);
  const files = preview?.files ?? [];
  const newFiles = files.filter(
    (file) => !file.is_dir && file.matched && !file.saved,
  ).length;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="w-[min(820px,calc(100vw-2rem))]">
        <DialogHeader>
          <DialogTitle>
            {mode === "savepath" ? "选择保存文件夹" : "正则处理预览"}
          </DialogTitle>
          <DialogDescription>
            {mode === "savepath"
              ? "目标目录不存在时会自动创建。选择后仍可在任务表单里手动修改。"
              : "预览本次分享中会被转存、重命名或跳过的文件。"}
          </DialogDescription>
        </DialogHeader>

        {mode === "savepath" ? (
          <DialogBody className="space-y-3">
            <div className="flex flex-wrap items-center gap-1.5 rounded-md border border-line bg-line-soft/40 px-2.5 py-2 text-[12px]">
              <button
                onClick={() => setPath("/")}
                className="flex items-center gap-1 rounded px-1.5 py-0.5 text-muted-ink hover:bg-panel hover:text-ink"
              >
                <Home className="h-3.5 w-3.5" />
                根目录
              </button>
              {breadcrumb.map((part, index) => {
                const nextPath = `/${breadcrumb.slice(0, index + 1).join("/")}`;
                return (
                  <span key={nextPath} className="flex items-center gap-1">
                    <ChevronRight className="h-3 w-3 text-subtle" />
                    <button
                      onClick={() => setPath(nextPath)}
                      className="rounded px-1.5 py-0.5 text-muted-ink hover:bg-panel hover:text-ink"
                    >
                      {part}
                    </button>
                  </span>
                );
              })}
            </div>

            <div className="flex items-center gap-2">
              <Input
                value={manualPath}
                onChange={(event) => setManualPath(event.target.value)}
                placeholder="/video/tv/..."
                aria-label="手动输入保存路径"
              />
              <Button
                variant="secondary"
                size="sm"
                onClick={() => {
                  const next = manualPath.startsWith("/")
                    ? manualPath
                    : `/${manualPath}`;
                  setPath(next);
                }}
              >
                前往
              </Button>
            </div>

            <div className="overflow-hidden rounded-md border border-line">
              <div className="flex items-center gap-2 border-b border-line bg-line-soft/50 px-3 py-2 font-mono text-[12px] text-muted-ink">
                <HardDrive className="h-3.5 w-3.5 shrink-0" />
                <span className="truncate">{path === "/" ? "根目录" : path}</span>
                <span className="ml-auto shrink-0 text-[12px] text-subtle">
                  仅显示文件夹
                </span>
              </div>
              {dirLoading ? (
                <div className="flex items-center justify-center gap-2 px-3 py-8 text-[12px] text-subtle">
                  <Loader2 className="h-3.5 w-3.5 animate-spin" />
                  正在读取网盘目录…
                </div>
              ) : dirError ? (
                <div className="px-3 py-8 text-center text-[12px] text-danger">
                  {dirError}
                </div>
              ) : entries.length === 0 ? (
                <div className="px-3 py-8 text-center text-[12px] text-muted-ink">
                  这个目录没有子文件夹。可以直接选择它作为保存路径，或返回上一级。
                </div>
              ) : (
                <div className="divide-y divide-line">
                  {entries.map((entry) => (
                    <div
                      key={entry.fid}
                      className="flex items-center gap-2.5 px-3 py-2.5 transition-colors hover:bg-line-soft/40"
                    >
                      <Folder
                        className="h-4 w-4 shrink-0 text-brand"
                        strokeWidth={1.6}
                      />
                      <button
                        className="min-w-0 flex-1 truncate text-left text-[13px] text-ink hover:underline"
                        onClick={() => {
                          const next =
                            path === "/" ? `/${entry.name}` : `${path}/${entry.name}`;
                          setPath(next);
                          setManualPath(next);
                        }}
                      >
                        {entry.name}
                      </button>
                      <span className="font-mono text-[12px] text-subtle">
                        {entry.count ?? 0} 项
                      </span>
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() =>
                          onPickPath(
                            path === "/" ? `/${entry.name}` : `${path}/${entry.name}`,
                          )
                        }
                      >
                        选择
                      </Button>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </DialogBody>
        ) : (
          <DialogBody className="space-y-3">
            <div className="flex flex-wrap items-center gap-2 rounded-md border border-line bg-line-soft/40 px-3 py-2">
              <Badge variant="info">
                <Sparkles className="h-3 w-3" />
                {preview?.pattern || task.pattern || "不筛选"}
              </Badge>
              <span className="font-mono text-[12px] text-muted-ink">
                → {preview?.replace || task.replace || "使用魔法变量"}
              </span>
              <span className="ml-auto font-mono text-[12px] text-subtle">
                {previewLoading ? "读取中…" : `${newFiles} 个新文件`}
              </span>
            </div>

            {previewError ? (
              <div className="rounded-md border border-danger/20 bg-danger-soft/40 px-3 py-6 text-center text-[12px] text-danger">
                {previewError}
              </div>
            ) : null}

            {previewLoading ? (
              <div className="flex items-center justify-center gap-2 rounded-md border border-line px-3 py-10 text-[12px] text-subtle">
                <Loader2 className="h-3.5 w-3.5 animate-spin" />
                正在读取分享内容并计算重命名结果…
              </div>
            ) : null}

            {!previewLoading && !previewError ? (
              <div className="overflow-hidden rounded-md border border-line">
                <div className="grid grid-cols-[1.5fr_1.5fr_70px_64px] items-center gap-3 border-b border-line bg-line-soft/50 px-3 py-2 font-mono text-[12px] uppercase tracking-[0.08em] text-muted-ink">
                  <span>分享文件</span>
                  <span>转存后名称</span>
                  <span>大小</span>
                  <span>状态</span>
                </div>
                <div className="max-h-[336px] divide-y divide-line overflow-y-auto">
                  {files.length === 0 ? (
                    <div className="px-3 py-8 text-center text-[12px] text-muted-ink">
                      这个分享是空的，或当前子目录没有文件。
                    </div>
                  ) : null}
                  {files.map((file) => (
                    <div
                      key={file.fid}
                      className="grid grid-cols-[1.5fr_1.5fr_70px_64px] items-center gap-3 px-3 py-2.5"
                    >
                      <div className="flex min-w-0 items-center gap-2">
                        {file.is_dir ? (
                          <Folder className="h-3.5 w-3.5 shrink-0 text-brand" />
                        ) : file.name.endsWith(".zip") ||
                          file.name.endsWith(".rar") ? (
                          <FileArchive className="h-3.5 w-3.5 shrink-0 text-warn" />
                        ) : (
                          <FileVideo className="h-3.5 w-3.5 shrink-0 text-muted-ink" />
                        )}
                        <span
                          className={cn(
                            "truncate text-[12px]",
                            file.matched ? "text-ink" : "text-subtle",
                          )}
                        >
                          {file.name}
                        </span>
                      </div>
                      <span
                        className={cn(
                          "truncate font-mono text-[12px]",
                          file.is_dir ? "text-subtle" : "text-ink-soft",
                        )}
                      >
                        {file.is_dir
                          ? "目录（不重命名）"
                          : file.matched
                            ? file.renamed || file.name
                            : "不匹配，跳过"}
                      </span>
                      <span className="font-mono text-[12px] text-subtle">
                        {file.is_dir ? "—" : formatBytes(file.size ?? 0)}
                      </span>
                      <span>
                        {file.is_dir ? (
                          <Badge variant="neutral">目录</Badge>
                        ) : !file.matched ? (
                          <Badge variant="neutral">跳过</Badge>
                        ) : file.saved ? (
                          <Badge variant="neutral">已存在</Badge>
                        ) : (
                          <Badge variant="ok">将转存</Badge>
                        )}
                      </span>
                    </div>
                  ))}
                </div>
              </div>
            ) : null}

            {preview && preview.total > files.length ? (
              <p className="text-[12px] text-subtle">
                共 {preview.total} 项，仅显示前 {files.length} 项。
              </p>
            ) : null}
            <p className="text-[12px] leading-4 text-muted-ink">
              目录与子目录内文件保持原名；普通文件按匹配表达式重命名。标记「已存在」的文件会被跳过。
            </p>
          </DialogBody>
        )}

        <DialogFooter>
          <Button variant="secondary" size="sm" onClick={() => onOpenChange(false)}>
            关闭
          </Button>
          {mode === "savepath" ? (
            <Button size="sm" onClick={() => onPickPath(path)}>
              <FolderOpen className="h-3.5 w-3.5" />
              选择此文件夹
            </Button>
          ) : (
            <Button
              size="sm"
              onClick={() => {
                notify("预览已确认，保存任务后按此规则转存", "ok");
                onOpenChange(false);
              }}
            >
              确认规则
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
