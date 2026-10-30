import { useState } from "react";
import { ChevronRight, Download, Pause, Play, X } from "lucide-react";
import type { DownloadJob } from "../../lib/types";
import { formatBytes } from "../../lib/mock";
import { cn } from "../../lib/utils";
import { Badge } from "../../components/ui/badge";
import { Button } from "../../components/ui/button";
import { ScrollArea } from "../../components/ui/scroll-area";
import { useDownloadQueue } from "./useDownloadQueue";

export function DownloadQueue() {
  const {
    jobs,
    activeJobs,
    finishedJobs,
    running,
    finishedFailed,
    finishedCanceled,
    pause,
    resume,
    cancel,
    clearFinished,
  } = useDownloadQueue();
  const [showFinished, setShowFinished] = useState(false);

  if (jobs.length === 0) return null;

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
              onClick={() => void pause(job.id)}
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
              onClick={() => void resume(job.id)}
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
              onClick={() => void cancel(job.id)}
            >
              <X className="h-3.5 w-3.5" />
            </Button>
          ) : null}
        </div>
        <div className="mt-1 truncate font-mono text-[11px] text-subtle">
          {job.root}
        </div>
        <div
          role="progressbar"
          aria-label={`${job.task_name} 下载进度`}
          aria-valuemin={0}
          aria-valuemax={100}
          aria-valuenow={percent}
          aria-valuetext={`${percent}%`}
          className="mt-1.5 h-1 overflow-hidden rounded-full bg-line-soft"
        >
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
        <div className={cn(activeJobs.length > 0 && "border-t border-line-soft")}>
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
  );
}
