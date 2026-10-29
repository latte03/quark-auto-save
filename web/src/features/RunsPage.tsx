import { useCallback, useEffect, useState } from "react";
import { Download, Loader2, RefreshCw, ScrollText, TriangleAlert } from "lucide-react";
import { getRunLog, getRuns } from "../lib/api";
import type { RunView } from "../lib/types";
import { cn } from "../lib/utils";
import { Badge } from "../components/ui/badge";
import { Button } from "../components/ui/button";
import { EmptyState } from "../components/EmptyState";
import { PageHeader } from "../components/PageHeader";
import { RunLogDialog } from "./RunLogDialog";

type RunsPageProps = {
  running: boolean;
  onOpenActive: () => void;
};

function formatTime(value?: string) {
  if (!value) return "—";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "—";
  return date.toLocaleString("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  });
}

function duration(run: RunView) {
  if (!run.finished_at) return "—";
  const start = new Date(run.started_at).getTime();
  const end = new Date(run.finished_at).getTime();
  if (Number.isNaN(start) || Number.isNaN(end) || end < start) return "—";
  const seconds = Math.round((end - start) / 1000);
  return seconds < 60 ? `${seconds}s` : `${Math.floor(seconds / 60)}m${seconds % 60}s`;
}

export function RunsPage({ running, onOpenActive }: RunsPageProps) {
  const [runs, setRuns] = useState<RunView[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [openRun, setOpenRun] = useState<RunView | null>(null);
  const [lines, setLines] = useState<string[]>([]);
  const [logLoading, setLogLoading] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const result = await getRuns();
      setRuns(result.runs ?? []);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "读取运行记录失败");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const viewRun = async (run: RunView) => {
    if (!run.done) {
      onOpenActive();
      return;
    }
    setOpenRun(run);
    setLines([]);
    setLogLoading(true);
    try {
      const result = await getRunLog(run.id);
      setLines(result.lines ?? []);
    } catch (cause) {
      setLines([cause instanceof Error ? cause.message : "读取日志失败"]);
    } finally {
      setLogLoading(false);
    }
  };

  return (
    <div className="space-y-4">
      <PageHeader
        title="运行记录"
        description="每次运行的结果、时长与日志。"
      />
      <section className="overflow-hidden rounded-panel bg-panel shadow-panel">
        <div className="flex items-center gap-2 px-3 py-2.5">
          <ScrollText className="h-4 w-4 text-muted-ink" strokeWidth={1.7} />
          <h2 className="text-[14px] font-semibold text-ink">运行日志</h2>
          <span className="ml-auto font-mono text-[12px] text-subtle">
            {loading ? "读取中" : `${runs.length} 条`}
          </span>
          <Button
            variant="ghost"
            size="icon-sm"
            aria-label="刷新运行日志"
            disabled={loading}
            onClick={() => void load()}
          >
            <RefreshCw className={cn("h-3.5 w-3.5", loading && "animate-spin")} />
          </Button>
        </div>

        {error ? (
          <EmptyState
            icon={TriangleAlert}
            title="无法读取运行日志"
            description={error}
            action={
              <Button variant="secondary" size="sm" onClick={() => void load()}>
                重试
              </Button>
            }
          />
        ) : runs.length === 0 && !loading ? (
          <EmptyState
            icon={Download}
            title="还没有运行日志"
            description="任务运行结束后，日志会保存在 NAS 上，只保留最近的若干条。"
          />
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full min-w-[620px] border-collapse text-left" aria-label="运行日志">
              <thead>
                <tr className="border-b border-line bg-line-soft/35">
                  <th className="h-8 px-3 text-[12px] font-medium text-muted-ink">开始</th>
                  <th className="h-8 px-3 text-[12px] font-medium text-muted-ink">任务</th>
                  <th className="h-8 w-[90px] px-3 text-[12px] font-medium text-muted-ink">结果</th>
                  <th className="h-8 w-[80px] px-3 text-right text-[12px] font-medium text-muted-ink">
                    新增
                  </th>
                  <th className="h-8 w-[80px] px-3 text-right text-[12px] font-medium text-muted-ink">
                    时长
                  </th>
                  <th className="h-8 w-[72px] px-3 text-right text-[12px] font-medium text-muted-ink">
                    日志
                  </th>
                </tr>
              </thead>
              <tbody>
                {runs.map((run) => (
                  <tr
                    key={run.id}
                    className="group border-b border-line-soft transition-colors last:border-b-0 hover:bg-brand-soft/25"
                  >
                    <td className="whitespace-nowrap px-3 py-2 font-mono text-[12px] text-muted-ink">
                      {formatTime(run.started_at)}
                    </td>
                    <td className="min-w-[140px] px-3 py-2">
                      <div className="flex min-w-0 items-center gap-1.5">
                        <span className="truncate text-[13px] text-ink">{run.label}</span>
                        {run.dry ? <Badge variant="warn">试运行</Badge> : null}
                        {!run.done ? <Badge variant="info">运行中</Badge> : null}
                      </div>
                    </td>
                    <td className="px-3 py-2">
                      {run.error ? (
                        <Badge variant="danger">失败</Badge>
                      ) : run.done ? (
                        <Badge variant="ok">完成</Badge>
                      ) : (
                        <Badge variant="info">进行中</Badge>
                      )}
                    </td>
                    <td className="px-3 py-2 text-right font-mono text-[12px] tabular-nums text-muted-ink">
                      {run.saved_total}
                    </td>
                    <td className="px-3 py-2 text-right font-mono text-[12px] tabular-nums text-muted-ink">
                      {running && !run.done ? "—" : duration(run)}
                    </td>
                    <td className="px-3 py-2 text-right">
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => void viewRun(run)}
                      >
                        {logLoading && openRun?.id === run.id ? (
                          <Loader2 className="h-3.5 w-3.5 animate-spin" />
                        ) : null}
                        查看
                      </Button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>

      <RunLogDialog
        open={openRun !== null}
        onOpenChange={(next) => {
          if (!next) setOpenRun(null);
        }}
        lines={lines}
        running={false}
        dry={openRun?.dry}
        allowRerun={false}
        onRunAgain={() => undefined}
      />
    </div>
  );
}
