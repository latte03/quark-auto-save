import { Fragment, useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  ArrowUp,
  ChevronRight,
  FileText,
  Folder,
  FolderPlus,
  Pencil,
  RefreshCw,
  Trash2,
} from "lucide-react";
import { browse, type BrowseEntry } from "../../lib/api";
import { formatBytes } from "../../lib/mock";
import { cn } from "../../lib/utils";
import { EmptyState } from "../../components/EmptyState";
import { Button } from "../../components/ui/button";
import { Skeleton } from "../../components/ui/skeleton";
import { useToast } from "../../components/ToastProvider";
import { FileDialogs } from "./FileDialogs";
import { categoryLabels, joinPath, parentPath } from "./paths";
import { useFileActions } from "./useFileActions";

const REFRESH_ATTEMPTS = 6;
const REFRESH_INTERVAL_MS = 250;

export function FileBrowser() {
  const notify = useToast();
  const [path, setPath] = useState("/");
  const [entries, setEntries] = useState<BrowseEntry[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  // 面包屑连点、刷新和改动后的轮询可能同时在飞，只认最新一次请求的结果。
  const requestSeq = useRef(0);

  const load = useCallback(
    async (target: string, until?: (list: BrowseEntry[]) => boolean) => {
      const requestId = ++requestSeq.current;
      setLoading(true);
      setError("");
      // Mutations poll with `until` and keep the current rows visible; plain
      // navigation swaps the skeleton in so the old directory never lingers.
      if (!until) setEntries([]);
      try {
        let result = await browse(target);
        // Quark's directory listing can trail a write by a moment; poll a few
        // times so create/rename/delete show up without a manual refresh.
        for (
          let attempt = 0;
          attempt < REFRESH_ATTEMPTS && until && !until(result.entries);
          attempt++
        ) {
          await new Promise((resolve) => setTimeout(resolve, REFRESH_INTERVAL_MS));
          if (requestId !== requestSeq.current) return;
          result = await browse(target);
        }
        if (requestId !== requestSeq.current) return;
        setEntries(result.entries);
        if (until && !until(result.entries)) {
          notify("列表还没刷出这次改动，稍后手动刷新看看", "warn");
        }
      } catch (cause) {
        if (requestId !== requestSeq.current) return;
        const message = cause instanceof Error ? cause.message : "读取目录失败";
        if (until) {
          // 刷新失败不该把已经显示的文件清空，给个非阻塞提示即可。
          notify(message, "warn");
        } else {
          setEntries([]);
          setError(message);
        }
      } finally {
        if (requestId === requestSeq.current) setLoading(false);
      }
    },
    [notify],
  );

  useEffect(() => {
    void load(path);
  }, [load, path]);

  const actions = useFileActions(path, load);

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

  return (
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
            aria-current={path === "/" ? "page" : undefined}
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
                  aria-current={current ? "page" : undefined}
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
          <Button variant="secondary" size="sm" onClick={actions.openMkdir}>
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
                      <div className="flex items-center justify-end gap-0.5 opacity-60 transition-opacity group-hover:opacity-100 group-focus-within:opacity-100">
                        <Button
                          variant="ghost"
                          size="icon-sm"
                          aria-label={`重命名 ${entry.name}`}
                          onClick={() => actions.openRename(entry)}
                        >
                          <Pencil className="h-3.5 w-3.5" />
                        </Button>
                        <Button
                          variant="ghost"
                          size="icon-sm"
                          className="text-danger hover:bg-danger-soft hover:text-danger"
                          aria-label={`删除 ${entry.name}`}
                          onClick={() => actions.setDeleteTarget(entry)}
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

      <FileDialogs path={path} actions={actions} />
    </section>
  );
}
