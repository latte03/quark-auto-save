import {
  Fragment,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import {
  ArrowUp,
  ChevronRight,
  Columns3,
  FileText,
  Folder,
  FolderPlus,
  List,
  Pencil,
  RefreshCw,
  Trash2,
} from "lucide-react";
import type { BrowseEntry } from "../../lib/api";
import { formatBytes } from "../../lib/mock";
import { cn } from "../../lib/utils";
import { EmptyState } from "../../components/EmptyState";
import { Button } from "../../components/ui/button";
import { Skeleton } from "../../components/ui/skeleton";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "../../components/ui/tooltip";
import { useToast } from "../../components/ToastProvider";
import { ColumnBrowser, type SelectedFile } from "./ColumnBrowser";
import { FileDialogs } from "./FileDialogs";
import { FileName } from "./FileName";
import { categoryLabels, joinPath, parentPath } from "./paths";
import {
  useDirectoryCache,
  useDirectoryState,
  type DirectoryCache,
} from "./useDirectoryCache";
import { useFileActions } from "./useFileActions";

type ViewMode = "list" | "columns";

const VIEW_KEY = "qas-files-view";

function readViewMode(): ViewMode {
  try {
    return window.localStorage.getItem(VIEW_KEY) === "columns"
      ? "columns"
      : "list";
  } catch {
    return "list";
  }
}

function saveViewMode(mode: ViewMode) {
  try {
    window.localStorage.setItem(VIEW_KEY, mode);
  } catch {
    // 隐私模式下写不进去，视图仍然可用。
  }
}

export function FileBrowser({
  cache: sharedCache,
}: {
  cache?: DirectoryCache;
} = {}) {
  const notify = useToast();
  const ownedCache = useDirectoryCache();
  const cache = sharedCache ?? ownedCache;
  const [mode, setMode] = useState<ViewMode>(readViewMode);
  const [path, setPath] = useState("/");
  const [selectedFile, setSelectedFile] = useState<SelectedFile | null>(null);
  const dir = useDirectoryState(cache, path);
  const errorSeen = useRef("");

  useEffect(() => {
    errorSeen.current = cache.get(path).error;
  }, [cache, path]);

  useEffect(() => {
    if (!dir.error) {
      errorSeen.current = "";
      return;
    }
    if (dir.entries.length === 0 || errorSeen.current === dir.error) return;
    errorSeen.current = dir.error;
    notify(dir.error, "warn");
  }, [dir.entries.length, dir.error, notify]);

  useEffect(() => {
    void cache.ensure(path);
  }, [cache, path]);

  const actions = useFileActions(path, cache);

  const sorted = useMemo(
    () =>
      [...dir.entries].sort((a, b) => {
        if (a.is_dir !== b.is_dir) return a.is_dir ? -1 : 1;
        return a.name.localeCompare(b.name, "zh");
      }),
    [dir.entries],
  );

  const segments = useMemo(
    () => (path === "/" ? [] : path.slice(1).split("/")),
    [path],
  );

  const navigate = (target: string) => {
    setSelectedFile(null);
    setPath(target);
  };

  const changeMode = (next: ViewMode) => {
    saveViewMode(next);
    setMode(next);
  };

  const selectFile = (parent: string, entry: BrowseEntry) => {
    setPath(parent);
    setSelectedFile({ dir: parent, entry });
  };

  const handleRenamed = (from: string, to: string) => {
    setSelectedFile(null);
    setPath((current) =>
      current === from || current.startsWith(`${from}/`)
        ? `${to}${current.slice(from.length)}`
        : current,
    );
  };

  const handleDeleted = (removed: string) => {
    setSelectedFile(null);
    setPath((current) =>
      current === removed || current.startsWith(`${removed}/`)
        ? parentPath(removed)
        : current,
    );
  };

  return (
    <section className="overflow-hidden rounded-panel bg-panel shadow-panel">
      <div className="flex flex-wrap items-center gap-2 px-3 py-2.5">
        {path !== "/" ? (
          <Button
            variant="ghost"
            size="icon-sm"
            aria-label="返回上级目录"
            onClick={() => navigate(parentPath(path))}
          >
            <ArrowUp className="h-3.5 w-3.5" />
          </Button>
        ) : null}
        <div className="flex min-w-0 flex-wrap items-center gap-0.5 text-[12px]">
          <button
            onClick={() => navigate("/")}
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
                  onClick={() => navigate(target)}
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
            {dir.loading ? "读取中" : `${sorted.length} 项`}
          </span>
          <Button
            variant="ghost"
            size="icon-sm"
            aria-label="刷新目录"
            disabled={dir.loading}
            onClick={() => void cache.refresh(path)}
          >
            <RefreshCw
              className={cn("h-3.5 w-3.5", dir.loading && "animate-spin")}
            />
          </Button>
          <div
            role="group"
            aria-label="视图切换"
            className="flex items-center gap-0.5 rounded-[min(var(--radius-md),10px)] border border-line bg-panel p-0.5"
          >
            <Tooltip>
              <TooltipTrigger
                render={
                  <Button
                    variant={mode === "list" ? "secondary" : "ghost"}
                    size="icon-sm"
                    aria-label="列表视图"
                    aria-pressed={mode === "list"}
                    onClick={() => changeMode("list")}
                  >
                    <List className="h-3.5 w-3.5" />
                  </Button>
                }
              />
              <TooltipContent>列表</TooltipContent>
            </Tooltip>
            <Tooltip>
              <TooltipTrigger
                render={
                  <Button
                    variant={mode === "columns" ? "secondary" : "ghost"}
                    size="icon-sm"
                    aria-label="分栏视图"
                    aria-pressed={mode === "columns"}
                    onClick={() => changeMode("columns")}
                  >
                    <Columns3 className="h-3.5 w-3.5" />
                  </Button>
                }
              />
              <TooltipContent>分栏</TooltipContent>
            </Tooltip>
          </div>
          <Button
            variant="secondary"
            size="sm"
            onClick={() => actions.openMkdir(path)}
          >
            <FolderPlus className="h-3.5 w-3.5" />
            新建文件夹
          </Button>
        </div>
      </div>

      {mode === "columns" ? (
        <ColumnBrowser
          path={path}
          cache={cache}
          actions={actions}
          selectedFile={selectedFile}
          onNavigate={navigate}
          onSelectFile={selectFile}
          onItemRenamed={handleRenamed}
          onItemDeleted={handleDeleted}
        />
      ) : dir.error && sorted.length === 0 ? (
        <EmptyState
          icon={FileText}
          title="无法读取这个目录"
          description={dir.error}
          action={
            <Button
              variant="secondary"
              size="sm"
              onClick={() => void cache.refresh(path)}
            >
              重试
            </Button>
          }
        />
      ) : dir.updatedAt === 0 && sorted.length === 0 ? (
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
          <table
            className="w-full min-w-[560px] table-fixed border-collapse text-left"
            aria-label="网盘文件"
          >
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
                    <td className="px-3 py-2">
                      <div className="flex min-w-0 items-center gap-2">
                        {entry.is_dir ? (
                          <Folder className="h-4 w-4 shrink-0 text-amber-500" strokeWidth={1.6} />
                        ) : (
                          <FileText className="h-4 w-4 shrink-0 text-subtle" strokeWidth={1.6} />
                        )}
                        <FileName
                          name={entry.name}
                          isDir={entry.is_dir}
                          onOpen={() => navigate(fullPath)}
                        />
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
                          onClick={() => actions.openDelete(entry)}
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

      <FileDialogs actions={actions} />
    </section>
  );
}
