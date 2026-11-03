import { useEffect, useMemo, useRef } from "react";
import {
  ChevronRight,
  FileText,
  Folder,
  Loader2,
  Pencil,
  RefreshCw,
  Trash2,
} from "lucide-react";
import type { BrowseEntry } from "../../lib/api";
import { formatBytes } from "../../lib/mock";
import { cn } from "../../lib/utils";
import { Button } from "../../components/ui/button";
import { Skeleton } from "../../components/ui/skeleton";
import { TruncatedText } from "./FileName";
import { categoryLabels, joinPath, parentPath } from "./paths";
import {
  useDirectoryState,
  type DirectoryCache,
} from "./useDirectoryCache";
import type { FileActions } from "./useFileActions";

export type SelectedFile = {
  dir: string;
  entry: BrowseEntry;
};

function baseName(path: string) {
  return path.slice(path.lastIndexOf("/") + 1);
}

function columnsOf(path: string) {
  const parts = path === "/" ? [] : path.slice(1).split("/");
  const columns = ["/"];
  for (let index = 0; index < parts.length; index += 1) {
    columns.push(`/${parts.slice(0, index + 1).join("/")}`);
  }
  return columns;
}

function sortEntries(entries: BrowseEntry[]) {
  return [...entries].sort((a, b) => {
    if (a.is_dir !== b.is_dir) return a.is_dir ? -1 : 1;
    return a.name.localeCompare(b.name, "zh");
  });
}

function ColumnSkeleton() {
  return (
    <div className="space-y-0.5 p-1">
      {[64, 82, 48, 72, 56].map((width) => (
        <div key={width} className="flex items-center gap-2 px-2 py-1.5">
          <Skeleton className="size-3.5 shrink-0 rounded" />
          <Skeleton className="h-3" style={{ width: `${width}%` }} />
        </div>
      ))}
    </div>
  );
}

function Column({
  path,
  cache,
  selectedName,
  onNavigate,
  onSelectFile,
}: {
  path: string;
  cache: DirectoryCache;
  selectedName: string | null;
  onNavigate: (target: string) => void;
  onSelectFile: (dir: string, entry: BrowseEntry) => void;
}) {
  const state = useDirectoryState(cache, path);
  const entries = useMemo(() => sortEntries(state.entries), [state.entries]);

  useEffect(() => {
    if (state.error) return;
    void cache.ensure(path);
  }, [cache, path, state.error, state.updatedAt]);

  const name = path === "/" ? "根目录" : baseName(path);

  return (
    <div className="flex h-full w-[240px] shrink-0 flex-col border-r border-line-soft last:border-r-0">
      <div className="flex h-8 shrink-0 items-center gap-1.5 border-b border-line bg-panel px-2.5 text-[12px]">
        <Folder className="h-3.5 w-3.5 shrink-0 text-amber-500" strokeWidth={1.7} />
        <span className="min-w-0 truncate font-medium text-ink">{name}</span>
        {state.loading ? (
          <Loader2 className="ml-auto h-3 w-3 shrink-0 animate-spin text-subtle" />
        ) : (
          <span className="ml-auto shrink-0 font-mono text-[11px] text-subtle">
            {entries.length}
          </span>
        )}
      </div>
      <div className="min-h-0 flex-1 overflow-y-auto p-1">
        {state.error && entries.length === 0 ? (
          <div className="m-1 rounded-md border border-danger/20 bg-danger-soft/40 px-2.5 py-2">
            <div className="text-[12px] leading-4 text-danger">{state.error}</div>
            <Button
              variant="ghost"
              size="sm"
              className="mt-1 h-6 px-1.5 text-danger hover:bg-danger-soft hover:text-danger"
              onClick={() => void cache.refresh(path)}
            >
              <RefreshCw className="h-3 w-3" />
              重试
            </Button>
          </div>
        ) : state.updatedAt === 0 ? (
          <ColumnSkeleton />
        ) : entries.length === 0 ? (
          <div className="px-3 py-6 text-center text-[12px] text-subtle">
            空目录
          </div>
        ) : (
          entries.map((entry) => {
            const selected = selectedName === entry.name;
            return (
              <button
                key={entry.fid || joinPath(path, entry.name)}
                type="button"
                aria-current={
                  selected ? (entry.is_dir ? "page" : "true") : undefined
                }
                onClick={() =>
                  entry.is_dir
                    ? onNavigate(joinPath(path, entry.name))
                    : onSelectFile(path, entry)
                }
                className={cn(
                  "flex w-full items-center gap-1.5 rounded-md px-2 py-1.5 text-left transition-colors",
                  selected
                    ? "bg-brand-soft text-brand-ink"
                    : "text-ink hover:bg-line-soft",
                )}
              >
                {entry.is_dir ? (
                  <Folder
                    className="h-3.5 w-3.5 shrink-0 text-amber-500"
                    strokeWidth={1.6}
                  />
                ) : (
                  <FileText
                    className={cn(
                      "h-3.5 w-3.5 shrink-0",
                      selected ? "text-brand-ink" : "text-subtle",
                    )}
                    strokeWidth={1.6}
                  />
                )}
                <TruncatedText
                  name={entry.name}
                  className="min-w-0 flex-1 truncate text-[12px]"
                />
                {entry.is_dir ? (
                  <ChevronRight
                    className={cn(
                      "h-3.5 w-3.5 shrink-0",
                      selected ? "text-brand-ink" : "text-subtle",
                    )}
                  />
                ) : (
                  <span className="shrink-0 font-mono text-[11px] text-subtle">
                    {formatBytes(entry.size)}
                  </span>
                )}
              </button>
            );
          })
        )}
      </div>
    </div>
  );
}

function Preview({
  path,
  cache,
  selectedFile,
  actions,
  onItemRenamed,
  onItemDeleted,
}: {
  path: string;
  cache: DirectoryCache;
  selectedFile: SelectedFile | null;
  actions: FileActions;
  onItemRenamed: (from: string, to: string) => void;
  onItemDeleted: (path: string) => void;
}) {
  const parent = path === "/" ? null : parentPath(path);
  const parentState = useDirectoryState(cache, parent ?? "/");
  const activeEntry =
    parent === null
      ? undefined
      : parentState.entries.find((entry) => entry.name === baseName(path));

  const item = selectedFile
    ? { entry: selectedFile.entry, dir: selectedFile.dir }
    : activeEntry && parent
      ? { entry: activeEntry, dir: parent }
      : null;

  return (
    <aside
      aria-label="详细信息"
      className="hidden w-[264px] shrink-0 flex-col border-l border-line-soft px-4 py-3.5 xl:flex"
    >
      <div className="font-mono text-[11px] tracking-[0.06em] text-subtle">
        详细信息
      </div>
      {item ? (
        <>
          <div className="mt-4 flex h-14 w-14 items-center justify-center rounded-xl bg-line-soft">
            {item.entry.is_dir ? (
              <Folder className="h-7 w-7 text-amber-500" strokeWidth={1.4} />
            ) : (
              <FileText className="h-7 w-7 text-subtle" strokeWidth={1.4} />
            )}
          </div>
          <div className="mt-3 w-full break-all text-[13px] font-medium leading-5 text-ink">
            {item.entry.name}
          </div>
          <div className="mt-1 text-[12px] text-muted-ink">
            {item.entry.is_dir
              ? "文件夹"
              : categoryLabels[item.entry.category ?? ""] ?? "文件"}
            {item.entry.is_dir
              ? item.entry.count
                ? ` · ${item.entry.count} 项`
                : ""
              : ` · ${formatBytes(item.entry.size)}`}
          </div>
          <div className="mt-4 w-full rounded-md border border-line bg-canvas/60 px-2.5 py-2 font-mono text-[11px] leading-4 break-all text-muted-ink">
            {joinPath(item.dir, item.entry.name)}
          </div>
          <div className="mt-auto flex items-center gap-2 pt-4">
            <Button
              variant="secondary"
              size="sm"
              onClick={() =>
                actions.openRename(item.entry, item.dir, onItemRenamed)
              }
            >
              <Pencil className="h-3.5 w-3.5" />
              重命名
            </Button>
            <Button
              variant="ghost"
              size="sm"
              className="text-danger hover:bg-danger-soft hover:text-danger"
              onClick={() =>
                actions.openDelete(item.entry, item.dir, onItemDeleted)
              }
            >
              <Trash2 className="h-3.5 w-3.5" />
              删除
            </Button>
          </div>
        </>
      ) : (
        <div className="flex flex-1 items-center justify-center px-4 text-center text-[12px] leading-5 text-subtle">
          选择文件或文件夹查看详情
        </div>
      )}
    </aside>
  );
}

export function ColumnBrowser({
  path,
  cache,
  actions,
  selectedFile,
  onNavigate,
  onSelectFile,
  onItemRenamed,
  onItemDeleted,
}: {
  path: string;
  cache: DirectoryCache;
  actions: FileActions;
  selectedFile: SelectedFile | null;
  onNavigate: (target: string) => void;
  onSelectFile: (dir: string, entry: BrowseEntry) => void;
  onItemRenamed: (from: string, to: string) => void;
  onItemDeleted: (path: string) => void;
}) {
  const scrollRef = useRef<HTMLDivElement>(null);
  const columns = useMemo(() => columnsOf(path), [path]);

  useEffect(() => {
    const element = scrollRef.current;
    if (!element) return;
    element.scrollTo({ left: element.scrollWidth, behavior: "auto" });
  }, [columns.length, path]);

  return (
    <div className="flex h-[clamp(340px,62vh,560px)] border-t border-line-soft">
      <div ref={scrollRef} className="flex min-w-0 flex-1 overflow-x-auto">
        {columns.map((columnPath, index) => {
          const next = columns[index + 1];
          const selectedName = next
            ? baseName(next)
            : selectedFile?.dir === columnPath
              ? selectedFile.entry.name
              : null;
          return (
            <Column
              key={columnPath}
              path={columnPath}
              cache={cache}
              selectedName={selectedName}
              onNavigate={onNavigate}
              onSelectFile={onSelectFile}
            />
          );
        })}
      </div>
      <Preview
        path={path}
        cache={cache}
        selectedFile={selectedFile}
        actions={actions}
        onItemRenamed={onItemRenamed}
        onItemDeleted={onItemDeleted}
      />
    </div>
  );
}
