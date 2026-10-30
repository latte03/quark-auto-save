import { useEffect, useState } from "react";
import { ChevronRight, FileArchive, FileVideo, Folder, Loader2 } from "lucide-react";
import type { SearchResult, Task } from "../lib/types";
import { formatBytes } from "../lib/mock";
import { shareDetail, type ShareDetail } from "../lib/api";
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

type SearchPreviewDialogProps = {
  open: boolean;
  hit: SearchResult | null;
  task: Task;
  onOpenChange: (open: boolean) => void;
  onUse: (hit: SearchResult) => void;
};

export function SearchPreviewDialog({
  open,
  hit,
  task,
  onOpenChange,
  onUse,
}: SearchPreviewDialogProps) {
  const [detail, setDetail] = useState<ShareDetail | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [path, setPath] = useState<{ fid: string; name: string }[]>([]);

  useEffect(() => {
    if (!open || !hit) return;
    let cancelled = false;
    setDetail(null);
    setError("");
    setLoading(true);
    shareDetail({
      shareurl: hit.shareurl,
      pdir_fid: path.length > 0 ? path[path.length - 1].fid : undefined,
      taskname: task.taskname,
      pattern: task.pattern,
      replace: task.replace,
      savepath: task.savepath,
      update_subdir: task.update_subdir,
      ignore_extension: task.ignore_extension,
    })
      .then((result) => {
        if (!cancelled) setDetail(result);
      })
      .catch((cause: unknown) => {
        if (!cancelled) {
          setError(cause instanceof Error ? cause.message : "读取分享内容失败");
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [
    open,
    hit,
    path,
    task.taskname,
    task.pattern,
    task.replace,
    task.savepath,
    task.update_subdir,
    task.ignore_extension,
  ]);

  useEffect(() => {
    if (!open) setPath([]);
  }, [open, hit]);

  const files = detail?.files ?? [];

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="w-[min(760px,calc(100vw-2rem))]">
        <DialogHeader>
          <DialogTitle>{hit?.taskname || "分享预览"}</DialogTitle>
          <DialogDescription>
            {hit?.content || hit?.shareurl || "读取该分享中的文件列表。"}
          </DialogDescription>
        </DialogHeader>

        <DialogBody className="space-y-3">
          <div className="flex flex-wrap items-center gap-2">
            <Badge variant="info">{hit?.source || "搜索结果"}</Badge>
            {hit?.channel ? (
              <span className="font-mono text-[12px] text-subtle">
                {hit.channel}
              </span>
            ) : null}
            {hit?.datetime ? (
              <span className="font-mono text-[12px] text-subtle">
                {hit.datetime}
              </span>
            ) : null}
            <span className="ml-auto font-mono text-[12px] text-subtle">
              {loading ? "读取中…" : `${files.length} 项`}
            </span>
          </div>

          <div className="flex flex-wrap items-center gap-1 rounded-md border border-line bg-line-soft/40 px-2.5 py-1.5 text-[12px]">
            <button
              type="button"
              onClick={() => setPath([])}
              className="rounded px-1.5 py-0.5 text-muted-ink hover:bg-panel hover:text-ink"
            >
              根目录
            </button>
            {path.map((item, index) => (
              <span key={item.fid} className="flex items-center gap-1">
                <ChevronRight className="h-3 w-3 text-subtle" />
                <button
                  type="button"
                  onClick={() => setPath(path.slice(0, index + 1))}
                  className="max-w-[180px] truncate rounded px-1.5 py-0.5 text-muted-ink hover:bg-panel hover:text-ink"
                >
                  {item.name}
                </button>
              </span>
            ))}
          </div>

          {error ? (
            <div className="rounded-md border border-danger/20 bg-danger-soft/40 px-3 py-6 text-center text-[12px] text-danger">
              {error}
            </div>
          ) : null}

          {loading ? (
            <div className="flex items-center justify-center gap-2 rounded-md border border-line px-3 py-10 text-[12px] text-subtle">
              <Loader2 className="h-3.5 w-3.5 animate-spin" />
              正在读取分享文件…
            </div>
          ) : null}

          {!loading && !error ? (
            <div className="overflow-hidden rounded-md border border-line">
              <div className="grid grid-cols-[1.7fr_88px_88px] items-center gap-3 border-b border-line bg-line-soft/50 px-3 py-2 font-mono text-[12px] uppercase tracking-[0.08em] text-muted-ink">
                <span>文件</span>
                <span>大小</span>
                <span>转存</span>
              </div>
              <div className="max-h-[320px] divide-y divide-line overflow-y-auto">
                {files.length === 0 ? (
                  <div className="px-3 py-8 text-center text-[12px] text-muted-ink">
                    这个分享是空的，或当前子目录没有文件。
                  </div>
                ) : null}
                {files.map((file) => {
                  const rowClass = cn(
                    "grid w-full grid-cols-[1.7fr_88px_88px] items-center gap-3 px-3 py-2.5 text-left",
                    file.is_dir && "hover:bg-line-soft/40",
                  );
                  const content = (
                    <>
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
                        {file.is_dir ? (
                          <ChevronRight className="h-3 w-3 shrink-0 text-subtle" />
                        ) : null}
                      </div>
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
                    </>
                  );
                  // 只有目录能点进去；文件行不是控件，不该被读成按钮。
                  return file.is_dir ? (
                    <button
                      key={file.fid}
                      type="button"
                      onClick={() =>
                        setPath([...path, { fid: file.fid, name: file.name }])
                      }
                      className={rowClass}
                    >
                      {content}
                    </button>
                  ) : (
                    <div key={file.fid} className={rowClass}>
                      {content}
                    </div>
                  );
                })}
              </div>
            </div>
          ) : null}

          {detail && detail.total > files.length ? (
            <p className="text-[12px] text-subtle">
              共 {detail.total} 项，仅显示前 {files.length} 项。
            </p>
          ) : null}
        </DialogBody>

        <DialogFooter>
          <Button variant="secondary" size="sm" onClick={() => onOpenChange(false)}>
            关闭
          </Button>
          <Button size="sm" disabled={!hit} onClick={() => hit && onUse(hit)}>
            使用此链接
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
