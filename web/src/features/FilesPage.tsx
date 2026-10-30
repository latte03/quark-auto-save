import { Fragment, useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  ArrowUp,
  ChevronRight,
  Download,
  FileText,
  Folder,
  FolderPlus,
  Loader2,
  Pause,
  Pencil,
  Play,
  RefreshCw,
  Trash2,
  X,
} from "lucide-react";
import {
  browse,
  cancelDownload,
  clearDownloads,
  createFolder,
  deleteFile,
  getDownloads,
  pauseDownload,
  renameFile,
  resumeDownload,
  type BrowseEntry,
} from "../lib/api";
import type { DownloadJob } from "../lib/types";
import { Badge } from "../components/ui/badge";
import { formatBytes } from "../lib/mock";
import { cn } from "../lib/utils";
import { EmptyState } from "../components/EmptyState";
import { PageHeader } from "../components/PageHeader";
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
import { ScrollArea } from "../components/ui/scroll-area";
import { Skeleton } from "../components/ui/skeleton";
import type { ToastTone } from "../components/Toast";

type FilesPageProps = {
  onNotify: (message: string, tone?: ToastTone) => void;
};

const categoryLabels: Record<string, string> = {
  video: "视频",
  image: "图片",
  audio: "音频",
  doc: "文档",
  archive: "压缩包",
};

function joinPath(base: string, name: string) {
  return base === "/" ? `/${name}` : `${base}/${name}`;
}

function parentPath(path: string) {
  const index = path.lastIndexOf("/");
  return index <= 0 ? "/" : path.slice(0, index);
}

export function FilesPage({ onNotify }: FilesPageProps) {
  const [path, setPath] = useState("/");
  const [entries, setEntries] = useState<BrowseEntry[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [renameTarget, setRenameTarget] = useState<BrowseEntry | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<BrowseEntry | null>(null);
  const [mkdirOpen, setMkdirOpen] = useState(false);
  const [nameInput, setNameInput] = useState("");
  const [busy, setBusy] = useState(false);
  const [jobs, setJobs] = useState<DownloadJob[]>([]);
  const [showFinished, setShowFinished] = useState(false);

  const downloadsActive = jobs.some(
    (job) => job.status === "queued" || job.status === "running",
  );
  const downloadsActiveRef = useRef(false);
  downloadsActiveRef.current = downloadsActive;

  // Poll fast while a job is running so the progress bar and speed move,
  // and back off once the queue is idle.
  useEffect(() => {
    let alive = true;
    let timer = 0;
    const tick = async () => {
      try {
        const result = await getDownloads();
        if (alive) setJobs(result.jobs ?? []);
      } catch {
        if (alive) setJobs([]);
      }
      if (alive) {
        timer = window.setTimeout(tick, downloadsActiveRef.current ? 1000 : 5000);
      }
    };
    void tick();
    return () => {
      alive = false;
      window.clearTimeout(timer);
    };
  }, []);

  const controlDownload = useCallback(
    async (action: (id: string) => Promise<unknown>, id: string) => {
      try {
        await action(id);
        const result = await getDownloads();
        setJobs(result.jobs ?? []);
      } catch (cause) {
        onNotify(cause instanceof Error ? cause.message : "操作失败", "error");
      }
    },
    [onNotify],
  );

  const clearFinished = useCallback(async () => {
    try {
      await clearDownloads();
      const result = await getDownloads();
      setJobs(result.jobs ?? []);
    } catch (cause) {
      onNotify(cause instanceof Error ? cause.message : "清空失败", "error");
    }
  }, [onNotify]);

  const load = useCallback(
    async (target: string, until?: (list: BrowseEntry[]) => boolean) => {
      setLoading(true);
      setError("");
      // Mutations poll with `until` and keep the current rows visible; plain
      // navigation swaps the skeleton in so the old directory never lingers.
      if (!until) setEntries([]);
      try {
        let result = await browse(target);
        // Quark's directory listing can trail a write by a moment; poll a few
        // times so create/rename/delete show up without a manual refresh.
        for (let attempt = 0; attempt < 6 && until && !until(result.entries); attempt++) {
          await new Promise((resolve) => setTimeout(resolve, 250));
          result = await browse(target);
        }
        setEntries(result.entries);
      } catch (cause) {
        setEntries([]);
        setError(cause instanceof Error ? cause.message : "读取目录失败");
      } finally {
        setLoading(false);
      }
    },
    [],
  );

  useEffect(() => {
    void load(path);
  }, [load, path]);

  const sorted = useMemo(
    () =>
      [...entries].sort((a, b) => {
        if (a.is_dir !== b.is_dir) return a.is_dir ? -1 : 1;
        return a.name.localeCompare(b.name, "zh");
      }),
    [entries],
  );

  const segments = useMemo(
    () => (path === "/" ? [] : path.slice(1).split("/")),
    [path],
  );

  const openRename = (entry: BrowseEntry) => {
    setNameInput(entry.name);
    setRenameTarget(entry);
  };

  const submitRename = async () => {
    if (!renameTarget) return;
    const name = nameInput.trim();
    if (!name) {
      onNotify("名称不能为空", "warn");
      return;
    }
    if (name === renameTarget.name) {
      setRenameTarget(null);
      return;
    }
    setBusy(true);
    try {
      await renameFile(joinPath(path, renameTarget.name), name);
      onNotify(`已重命名为 ${name}`, "ok");
      setRenameTarget(null);
      await load(path, (list) => list.some((entry) => entry.name === name));
    } catch (cause) {
      onNotify(cause instanceof Error ? cause.message : "重命名失败", "error");
    } finally {
      setBusy(false);
    }
  };

  const submitDelete = async () => {
    if (!deleteTarget) return;
    setBusy(true);
    try {
      await deleteFile(joinPath(path, deleteTarget.name));
      onNotify(`已移入回收站：${deleteTarget.name}`, "ok");
      const removedFID = deleteTarget.fid;
      setDeleteTarget(null);
      await load(path, (list) => !list.some((entry) => entry.fid === removedFID));
    } catch (cause) {
      onNotify(cause instanceof Error ? cause.message : "删除失败", "error");
    } finally {
      setBusy(false);
    }
  };

  const submitMkdir = async () => {
    const name = nameInput.trim();
    if (!name) {
      onNotify("文件夹名称不能为空", "warn");
      return;
    }
    if (name.includes("/")) {
      onNotify("文件夹名称不能包含 /", "warn");
      return;
    }
    setBusy(true);
    try {
      await createFolder(joinPath(path, name));
      onNotify(`已新建文件夹 ${name}`, "ok");
      setMkdirOpen(false);
      await load(path, (list) => list.some((entry) => entry.name === name));
    } catch (cause) {
      onNotify(cause instanceof Error ? cause.message : "新建文件夹失败", "error");
    } finally {
      setBusy(false);
    }
  };

  const activeJobs = jobs.filter(
    (job) =>
      job.status === "queued" ||
      job.status === "running" ||
      job.status === "paused",
  );
  const finishedJobs = jobs.filter((job) => !activeJobs.includes(job));
  const running = activeJobs.filter(
    (job) => job.status === "queued" || job.status === "running",
  ).length;
  const finishedFailed = finishedJobs.filter(
    (job) => job.status === "failed",
  ).length;
  const finishedCanceled = finishedJobs.filter(
    (job) => job.status === "canceled",
  ).length;

  const renderJob = (job: DownloadJob) => {
    const handled = job.done + job.skipped;
    const percent =
      job.total_bytes && job.total_bytes > 0
        ? job.status === "done"
          ? 100
          : Math.min(99, Math.round((job.bytes / job.total_bytes) * 100))
        : job.total > 0
          ? Math.round((handled / job.total) * 100)
          : 0;
    const label =
      job.status === "queued"
        ? "排队中"
        : job.status === "running"
          ? "下载中"
          : job.status === "paused"
            ? "已暂停"
            : job.status === "canceled"
              ? "已取消"
              : job.status === "failed"
                ? "失败"
                : "已完成";
    const controllable =
      job.status === "queued" ||
      job.status === "running" ||
      job.status === "paused";
    return (
      <div key={job.id} className="px-3 py-2.5">
        <div className="flex flex-wrap items-center gap-2">
          <span className="truncate text-[13px] font-medium text-ink">
            {job.task_name}
          </span>
          <Badge
            variant={
              job.status === "failed"
                ? "danger"
                : job.status === "done"
                  ? "ok"
                  : job.status === "paused"
                    ? "warn"
                    : job.status === "canceled"
                      ? "neutral"
                      : "info"
            }
          >
            {label}
          </Badge>
          <span className="ml-auto font-mono text-[12px] tabular-nums text-subtle">
            {handled}/{job.total} · {formatBytes(job.bytes)}
            {job.status === "running" && (job.speed ?? 0) > 0
              ? ` · ${formatBytes(job.speed ?? 0)}/s`
              : ""}
            {job.failed > 0 ? ` · 失败 ${job.failed}` : ""}
          </span>
          {job.status === "queued" || job.status === "running" ? (
            <Button
              variant="ghost"
              size="icon-sm"
              aria-label={`暂停 ${job.task_name}`}
              title="暂停"
              onClick={() => void controlDownload(pauseDownload, job.id)}
            >
              <Pause className="h-3.5 w-3.5" />
            </Button>
          ) : null}
          {job.status === "paused" ? (
            <Button
              variant="ghost"
              size="icon-sm"
              aria-label={`继续 ${job.task_name}`}
              title="继续"
              onClick={() => void controlDownload(resumeDownload, job.id)}
            >
              <Play className="h-3.5 w-3.5" />
            </Button>
          ) : null}
          {controllable ? (
            <Button
              variant="ghost"
              size="icon-sm"
              aria-label={`取消 ${job.task_name}`}
              title="取消"
              onClick={() => void controlDownload(cancelDownload, job.id)}
            >
              <X className="h-3.5 w-3.5" />
            </Button>
          ) : null}
        </div>
        <div className="mt-1 truncate font-mono text-[11px] text-subtle">
          {job.root}
        </div>
        <div className="mt-1.5 h-1 overflow-hidden rounded-full bg-line-soft">
          <div
            className="h-full origin-left rounded-full bg-brand transition-transform duration-500 ease-out"
            style={{ transform: `scaleX(${percent / 100})` }}
          />
        </div>
        {job.message ? (
          <div
            className={cn(
              "mt-1 truncate text-[11px]",
              job.status === "failed" ? "text-danger" : "text-muted-ink",
            )}
          >
            {job.message}
          </div>
        ) : null}
      </div>
    );
  };

  return (
    <div className="space-y-4">
      <PageHeader
        title="文件"
        description="夸克网盘目录与本地下载队列。"
      />
      {jobs.length > 0 ? (
        <section className="overflow-hidden rounded-panel bg-panel shadow-panel">
          <div className="flex items-center gap-2 px-3 py-2.5">
            <Download className="h-4 w-4 text-muted-ink" strokeWidth={1.7} />
            <h2 className="text-[14px] font-semibold text-ink">下载队列</h2>
            <span className="ml-auto font-mono text-[12px] text-subtle">
              {running > 0 ? `${running} 个进行中` : "已空闲"}
            </span>
          </div>
          {activeJobs.length > 0 ? (
            <div className="divide-y divide-line-soft">
              {activeJobs.map(renderJob)}
            </div>
          ) : null}
          {finishedJobs.length > 0 ? (
            <div
              className={cn(activeJobs.length > 0 && "border-t border-line-soft")}
            >
              <div className="flex items-center gap-1 px-3 py-1.5">
                <button
                  type="button"
                  aria-expanded={showFinished}
                  onClick={() => setShowFinished((value) => !value)}
                  className="flex items-center gap-1.5 rounded px-1 py-1 text-[12px] text-muted-ink transition-colors hover:text-ink"
                >
                  <ChevronRight
                    className={cn(
                      "h-3.5 w-3.5 transition-transform",
                      showFinished && "rotate-90",
                    )}
                  />
                  已结束 {finishedJobs.length} 项
                  {finishedFailed > 0 ? (
                    <span className="text-danger">· 失败 {finishedFailed}</span>
                  ) : null}
                  {finishedCanceled > 0 ? (
                    <span>· 取消 {finishedCanceled}</span>
                  ) : null}
                </button>
                <Button
                  variant="ghost"
                  size="xs"
                  className="ml-auto text-muted-ink"
                  onClick={() => void clearFinished()}
                >
                  清空
                </Button>
              </div>
              {showFinished ? (
                <ScrollArea className="max-h-[240px] min-h-0 border-t border-line-soft">
                  <div className="divide-y divide-line-soft">
                    {finishedJobs.map(renderJob)}
                  </div>
                </ScrollArea>
              ) : null}
            </div>
          ) : null}
        </section>
      ) : null}
      <section className="overflow-hidden rounded-panel bg-panel shadow-panel">
        <div className="flex flex-wrap items-center gap-2 px-3 py-2.5">
          {path !== "/" ? (
            <Button
              variant="ghost"
              size="icon-sm"
              aria-label="返回上级目录"
              onClick={() => setPath(parentPath(path))}
            >
              <ArrowUp className="h-3.5 w-3.5" />
            </Button>
          ) : null}
          <div className="flex min-w-0 flex-wrap items-center gap-0.5 text-[12px]">
            <button
              onClick={() => setPath("/")}
              className={cn(
                "rounded px-1.5 py-1 transition-colors hover:bg-line-soft",
                path === "/" ? "font-medium text-ink" : "text-muted-ink hover:text-ink",
              )}
            >
              根目录
            </button>
            {segments.map((segment, index) => {
              const target = `/${segments.slice(0, index + 1).join("/")}`;
              const current = index === segments.length - 1;
              return (
                <Fragment key={target}>
                  <ChevronRight className="h-3 w-3 shrink-0 text-subtle" />
                  <button
                    onClick={() => setPath(target)}
                    className={cn(
                      "max-w-[220px] truncate rounded px-1.5 py-1 transition-colors hover:bg-line-soft",
                      current
                        ? "font-medium text-ink"
                        : "text-muted-ink hover:text-ink",
                    )}
                  >
                    {segment}
                  </button>
                </Fragment>
              );
            })}
          </div>
          <div className="ml-auto flex items-center gap-1.5">
            <span className="mr-1 font-mono text-[12px] text-subtle">
              {loading ? "读取中" : `${sorted.length} 项`}
            </span>
            <Button
              variant="ghost"
              size="icon-sm"
              aria-label="刷新目录"
              disabled={loading}
              onClick={() => void load(path)}
            >
              <RefreshCw className={cn("h-3.5 w-3.5", loading && "animate-spin")} />
            </Button>
            <Button
              variant="secondary"
              size="sm"
              onClick={() => {
                setNameInput("");
                setMkdirOpen(true);
              }}
            >
              <FolderPlus className="h-3.5 w-3.5" />
              新建文件夹
            </Button>
          </div>
        </div>

        {error ? (
          <EmptyState
            icon={FileText}
            title="无法读取这个目录"
            description={error}
            action={
              <Button variant="secondary" size="sm" onClick={() => void load(path)}>
                重试
              </Button>
            }
          />
        ) : loading && sorted.length === 0 ? (
          <div aria-busy="true" aria-label="正在读取目录" className="divide-y divide-line-soft">
            {[46, 30, 38, 24, 52, 34, 42, 28].map((width, index) => (
              <div key={index} className="flex items-center gap-3 px-3 py-[9px]">
                <Skeleton className="size-4 shrink-0 rounded" />
                <Skeleton className="h-3.5" style={{ width: `${width}%` }} />
                <Skeleton className="ms-auto hidden h-3 w-12 shrink-0 sm:block" />
                <Skeleton className="h-3 w-14 shrink-0" />
                <Skeleton className="h-3 w-10 shrink-0" />
              </div>
            ))}
          </div>
        ) : sorted.length === 0 ? (
          <EmptyState
            icon={Folder}
            title="这个目录是空的"
            description="转存任务会在这里创建文件；也可以先新建一个文件夹。"
          />
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full min-w-[560px] border-collapse text-left" aria-label="网盘文件">
              <thead>
                <tr className="border-b border-line bg-line-soft/35">
                  <th className="h-8 px-3 text-[12px] font-medium text-muted-ink">名称</th>
                  <th className="h-8 w-[110px] px-3 text-[12px] font-medium text-muted-ink">
                    类型
                  </th>
                  <th className="h-8 w-[110px] px-3 text-right text-[12px] font-medium text-muted-ink">
                    大小
                  </th>
                  <th className="h-8 w-[96px] px-3 text-right text-[12px] font-medium text-muted-ink">
                    操作
                  </th>
                </tr>
              </thead>
              <tbody>
                {sorted.map((entry) => {
                  const fullPath = joinPath(path, entry.name);
                  return (
                    <tr
                      key={entry.fid || fullPath}
                      className="group border-b border-line-soft transition-colors last:border-b-0 hover:bg-brand-soft/25"
                    >
                      <td className="min-w-[160px] px-3 py-2">
                        <div className="flex min-w-0 items-center gap-2">
                          {entry.is_dir ? (
                            <Folder className="h-4 w-4 shrink-0 text-amber-500" strokeWidth={1.6} />
                          ) : (
                            <FileText className="h-4 w-4 shrink-0 text-subtle" strokeWidth={1.6} />
                          )}
                          {entry.is_dir ? (
                            <button
                              onClick={() => setPath(fullPath)}
                              className="truncate text-left text-[13px] font-medium text-ink hover:text-brand-ink hover:underline"
                            >
                              {entry.name}
                            </button>
                          ) : (
                            <span className="truncate text-[13px] text-ink">{entry.name}</span>
                          )}
                        </div>
                      </td>
                      <td className="px-3 py-2 text-[12px] text-muted-ink">
                        {entry.is_dir
                          ? "文件夹"
                          : categoryLabels[entry.category ?? ""] ?? "文件"}
                      </td>
                      <td className="px-3 py-2 text-right font-mono text-[12px] tabular-nums text-muted-ink">
                        {entry.is_dir ? "—" : formatBytes(entry.size)}
                      </td>
                      <td className="px-3 py-2">
                        <div className="flex items-center justify-end gap-0.5 opacity-60 transition-opacity group-hover:opacity-100">
                          <Button
                            variant="ghost"
                            size="icon-sm"
                            aria-label={`重命名 ${entry.name}`}
                            onClick={() => openRename(entry)}
                          >
                            <Pencil className="h-3.5 w-3.5" />
                          </Button>
                          <Button
                            variant="ghost"
                            size="icon-sm"
                            className="text-danger hover:bg-danger-soft hover:text-danger"
                            aria-label={`删除 ${entry.name}`}
                            onClick={() => setDeleteTarget(entry)}
                          >
                            <Trash2 className="h-3.5 w-3.5" />
                          </Button>
                        </div>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </section>

      <Dialog
        open={Boolean(renameTarget)}
        onOpenChange={(open) => {
          if (!open) setRenameTarget(null);
        }}
      >
        <DialogContent className="w-[min(440px,calc(100vw-2rem))]">
          <DialogHeader>
            <DialogTitle>重命名</DialogTitle>
            <DialogDescription>
              当前路径：{renameTarget ? joinPath(path, renameTarget.name) : "—"}
            </DialogDescription>
          </DialogHeader>
          <DialogBody>
            <Input
              autoFocus
              value={nameInput}
              onChange={(event) => setNameInput(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === "Enter") void submitRename();
              }}
              aria-label="新名称"
            />
          </DialogBody>
          <DialogFooter>
            <Button variant="secondary" size="sm" onClick={() => setRenameTarget(null)}>
              取消
            </Button>
            <Button size="sm" disabled={busy} onClick={() => void submitRename()}>
              {busy ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : null}
              保存
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog
        open={Boolean(deleteTarget)}
        onOpenChange={(open) => {
          if (!open) setDeleteTarget(null);
        }}
      >
        <DialogContent className="w-[min(420px,calc(100vw-2rem))]">
          <DialogHeader>
            <DialogTitle>删除文件</DialogTitle>
            <DialogDescription>
              文件会移入夸克网盘回收站，可在网盘侧恢复。
            </DialogDescription>
          </DialogHeader>
          <DialogBody>
            <div className="rounded-md border border-danger/20 bg-danger-soft/40 px-3 py-2.5 text-[13px] text-ink">
              确定删除「{deleteTarget?.name}
              {deleteTarget?.is_dir ? "」（含目录内全部文件）" : "」"}？
            </div>
          </DialogBody>
          <DialogFooter>
            <Button variant="secondary" size="sm" onClick={() => setDeleteTarget(null)}>
              取消
            </Button>
            <Button variant="destructive" size="sm" disabled={busy} onClick={() => void submitDelete()}>
              {busy ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : null}
              删除
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={mkdirOpen} onOpenChange={setMkdirOpen}>
        <DialogContent className="w-[min(440px,calc(100vw-2rem))]">
          <DialogHeader>
            <DialogTitle>新建文件夹</DialogTitle>
            <DialogDescription>创建在 {path}</DialogDescription>
          </DialogHeader>
          <DialogBody>
            <Input
              autoFocus
              value={nameInput}
              placeholder="文件夹名称"
              onChange={(event) => setNameInput(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === "Enter") void submitMkdir();
              }}
              aria-label="文件夹名称"
            />
          </DialogBody>
          <DialogFooter>
            <Button variant="secondary" size="sm" onClick={() => setMkdirOpen(false)}>
              取消
            </Button>
            <Button size="sm" disabled={busy} onClick={() => void submitMkdir()}>
              {busy ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : null}
              创建
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
