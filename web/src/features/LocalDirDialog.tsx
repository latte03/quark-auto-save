import { useEffect, useState } from "react";
import {
  ChevronRight,
  Folder,
  FolderOpen,
  HardDrive,
  Loader2,
  Plus,
} from "lucide-react";
import { browseLocalDirs, type LocalDirBrowse } from "../lib/api";
import { pickSharedDirectory } from "../lib/trim";
import { cn } from "../lib/utils";
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
import { useToast } from "../components/ToastProvider";

type LocalDirDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  initial?: string;
  onSelect: (path: string) => void;
};

// Pick a folder inside the administrator-approved roots; new authorisation is
// only a shortcut here, the main path is browsing what is already granted.
export function LocalDirDialog({
  open,
  onOpenChange,
  initial,
  onSelect,
}: LocalDirDialogProps) {
  const notify = useToast();
  const [data, setData] = useState<LocalDirBrowse | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [authorizing, setAuthorizing] = useState(false);

  useEffect(() => {
    if (!open) return;
    let cancelled = false;
    setData(null);
    setError("");
    setLoading(true);
    const load = async () => {
      try {
        const result = await browseLocalDirs(initial?.trim() || undefined);
        if (!cancelled) setData(result);
      } catch {
        try {
          const result = await browseLocalDirs();
          if (!cancelled) setData(result);
        } catch (cause) {
          if (!cancelled) {
            setError(cause instanceof Error ? cause.message : "读取授权目录失败");
          }
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    };
    void load();
    return () => {
      cancelled = true;
    };
  }, [open, initial]);

  const loadPath = async (path?: string) => {
    setLoading(true);
    setError("");
    try {
      setData(await browseLocalDirs(path));
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "读取目录失败");
    } finally {
      setLoading(false);
    }
  };

  const authorize = async () => {
    setAuthorizing(true);
    try {
      const dir = await pickSharedDirectory();
      if (!dir) return;
      notify(`已授权 ${dir}`, "ok");
      await loadPath(dir);
    } catch (cause) {
      notify(
        cause instanceof Error ? cause.message : "无法打开飞牛目录选择器",
        "warn",
      );
    } finally {
      setAuthorizing(false);
    }
  };

  const roots = data?.roots ?? [];
  const path = data?.path ?? "";
  const root = roots.find(
    (item) => path === item || path.startsWith(`${item}/`),
  );
  const segments =
    root && path !== root ? path.slice(root.length + 1).split("/") : [];

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="w-[min(560px,calc(100vw-2rem))]">
        <DialogHeader>
          <DialogTitle>选择下载目录</DialogTitle>
          <DialogDescription>
            在管理员已授权的目录里选择子目录；需要新的目录时再用「新增授权」。
          </DialogDescription>
        </DialogHeader>

        <div className="flex min-w-0 flex-wrap items-center gap-0.5 border-b border-line px-4 py-2 text-[12px]">
          <button
            onClick={() => void loadPath()}
            className={cn(
              "rounded px-1.5 py-1 transition-colors hover:bg-line-soft",
              path === "" ? "font-medium text-ink" : "text-muted-ink hover:text-ink",
            )}
          >
            已授权目录
          </button>
          {root ? (
            <>
              <ChevronRight className="h-3 w-3 shrink-0 text-subtle" />
              <button
                onClick={() => void loadPath(root)}
                className={cn(
                  "max-w-[220px] truncate rounded px-1.5 py-1 transition-colors hover:bg-line-soft",
                  path === root ? "font-medium text-ink" : "text-muted-ink hover:text-ink",
                )}
              >
                {root}
              </button>
            </>
          ) : null}
          {segments.map((segment, index) => {
            const target = `${root}/${segments.slice(0, index + 1).join("/")}`;
            const current = index === segments.length - 1;
            return (
              <span key={target} className="flex min-w-0 items-center">
                <ChevronRight className="h-3 w-3 shrink-0 text-subtle" />
                <button
                  onClick={() => void loadPath(target)}
                  className={cn(
                    "max-w-[160px] truncate rounded px-1.5 py-1 transition-colors hover:bg-line-soft",
                    current ? "font-medium text-ink" : "text-muted-ink hover:text-ink",
                  )}
                >
                  {segment}
                </button>
              </span>
            );
          })}
        </div>

        <DialogBody className="min-h-[220px] p-0">
          {loading ? (
            <div className="flex items-center justify-center gap-2 py-12 text-[12px] text-muted-ink">
              <Loader2 className="h-4 w-4 animate-spin" />
              正在读取…
            </div>
          ) : error ? (
            <div className="px-4 py-10 text-center text-[12px] text-danger">
              {error}
            </div>
          ) : path === "" ? (
            roots.length === 0 ? (
              <div className="px-4 py-10 text-center text-[12px] text-muted-ink">
                还没有已授权目录，点击「新增授权」让管理员选择一个。
              </div>
            ) : (
              <div className="divide-y divide-line-soft">
                {roots.map((item) => (
                  <button
                    key={item}
                    onClick={() => void loadPath(item)}
                    className="flex w-full items-center gap-2 px-4 py-2.5 text-left transition-colors hover:bg-brand-soft/25"
                  >
                    <HardDrive className="h-4 w-4 shrink-0 text-muted-ink" strokeWidth={1.6} />
                    <span className="truncate font-mono text-[12px] text-ink">{item}</span>
                    <span className="ml-auto shrink-0 text-[11px] text-subtle">已授权</span>
                  </button>
                ))}
              </div>
            )
          ) : (
            <div className="divide-y divide-line-soft">
              {data?.parent ? (
                <button
                  onClick={() => void loadPath(data.parent || undefined)}
                  className="flex w-full items-center gap-2 px-4 py-2.5 text-left transition-colors hover:bg-line-soft/40"
                >
                  <FolderOpen className="h-4 w-4 shrink-0 text-amber-500" strokeWidth={1.6} />
                  <span className="text-[13px] text-muted-ink">上一级</span>
                </button>
              ) : null}
              {data?.dirs.length === 0 ? (
                <div className="px-4 py-10 text-center text-[12px] text-muted-ink">
                  没有子目录，可以直接使用当前目录。
                </div>
              ) : (
                data?.dirs.map((dir) => (
                  <button
                    key={dir.path}
                    onClick={() => void loadPath(dir.path)}
                    className="flex w-full items-center gap-2 px-4 py-2.5 text-left transition-colors hover:bg-brand-soft/25"
                  >
                    <Folder className="h-4 w-4 shrink-0 text-amber-500" strokeWidth={1.6} />
                    <span className="truncate text-[13px] text-ink">{dir.name}</span>
                  </button>
                ))
              )}
            </div>
          )}
        </DialogBody>

        <DialogFooter className="flex-wrap gap-2">
          <Button
            variant="ghost"
            size="sm"
            disabled={authorizing}
            onClick={() => void authorize()}
          >
            {authorizing ? (
              <Loader2 className="h-3.5 w-3.5 animate-spin" />
            ) : (
              <Plus className="h-3.5 w-3.5" />
            )}
            新增授权
          </Button>
          <span className="mr-auto truncate font-mono text-[11px] text-subtle">
            {path || "未选择"}
          </span>
          <Button variant="secondary" size="sm" onClick={() => onOpenChange(false)}>
            取消
          </Button>
          <Button
            size="sm"
            disabled={!path || loading}
            onClick={() => {
              onSelect(path);
              onOpenChange(false);
            }}
          >
            使用此目录
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
