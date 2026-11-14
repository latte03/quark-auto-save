import { useEffect, useMemo, useState } from "react";
import {
  ArrowRight,
  CalendarClock,
  CheckCircle2,
  Clock3,
  Play,
  TriangleAlert,
  Wrench,
} from "lucide-react";
import { getRuns } from "../lib/api";
import type { RunView, Task } from "../lib/types";
import { formatDateTime } from "../lib/format";
import { cn } from "../lib/utils";
import { AlertStrip } from "../components/AlertStrip";
import { PageHeader } from "../components/PageHeader";
import { PageSection } from "../components/PageSection";
import { TaskTable } from "../components/TaskTable";
import { TaskStatusBadge } from "../components/TaskStatusBadge";
import { AnimatedNumber } from "../components/ui/animated-number";
import { Button } from "../components/ui/button";

type OverviewPageProps = {
  tasks: Task[];
  bannedCount: number;
  latestRun: RunView | null;
  onOpenTask: (task: Task) => void;
  onRunTask: (task: Task) => void;
  onDeleteTask: (task: Task) => void;
  onRepairTask: (task: Task) => void;
  onToggleTask: (task: Task, disabled: boolean) => void;
  onOpenLog: () => void;
  onRunAll: () => void;
  onGoTasks: (filter?: "banned") => void;
};

function runSeconds(run: RunView | null) {
  if (!run?.finished_at) return null;
  const elapsed =
    (new Date(run.finished_at).getTime() - new Date(run.started_at).getTime()) / 1000;
  return Number.isFinite(elapsed) && elapsed >= 0 ? elapsed : null;
}

/** 最近 7 天新增文件的小条形图；write 运行按天聚合。 */
function TrendBars({ runs }: { runs: RunView[] }) {
  const days = useMemo(() => {
    const buckets: { label: string; value: number; key: string }[] = [];
    const today = new Date();
    for (let offset = 6; offset >= 0; offset--) {
      const date = new Date(today);
      date.setDate(today.getDate() - offset);
      buckets.push({
        label: `${date.getMonth() + 1}/${date.getDate()}`,
        key: `${date.getFullYear()}-${date.getMonth()}-${date.getDate()}`,
        value: 0,
      });
    }
    const index = new Map(buckets.map((bucket, i) => [bucket.key, i]));
    for (const run of runs) {
      if (run.dry) continue;
      const date = new Date(run.started_at);
      const key = `${date.getFullYear()}-${date.getMonth()}-${date.getDate()}`;
      const bucket = index.get(key);
      if (bucket != null) buckets[bucket].value += run.saved_total;
    }
    return buckets;
  }, [runs]);
  const max = Math.max(1, ...days.map((day) => day.value));

  return (
    <div
      aria-label="近 7 天新增文件"
      className="flex w-full max-w-[240px] items-end gap-1.5"
    >
      {days.map((day, i) => {
        const height =
          day.value > 0 ? Math.max(12, (day.value / max) * 100) : 4;
        return (
          <div
            key={day.key}
            className="flex min-w-0 flex-1 flex-col items-center gap-1"
            title={`${day.label}：${day.value} 个文件`}
          >
            <div className="flex h-12 w-full items-end">
              <div
                className={cn(
                  "w-full rounded-t-[3px] transition-[height] duration-500",
                  day.value > 0 ? "bg-brand" : "bg-line",
                  i === days.length - 1 && day.value > 0 && "bg-brand/80",
                )}
                style={{ height: `${height}%` }}
              />
            </div>
            <span className="font-mono text-[10px] text-subtle">{day.label}</span>
          </div>
        );
      })}
    </div>
  );
}

export function OverviewPage({
  tasks,
  bannedCount,
  latestRun,
  onOpenTask,
  onRunTask,
  onDeleteTask,
  onRepairTask,
  onToggleTask,
  onOpenLog,
  onRunAll,
  onGoTasks,
}: OverviewPageProps) {
  const banned = tasks.filter((task) => task.status === "banned");
  const upcoming = tasks
    .filter((task) => task.status === "tracking" || task.status === "waiting")
    .slice(0, 4);
  const recent = tasks.slice(0, 5);
  const tracking = tasks.filter((task) => task.status === "tracking").length;
  const done = tasks.filter(
    (task) => task.status === "done" || task.status === "once_done",
  ).length;
  const seconds = runSeconds(latestRun);
  const failedTasks = latestRun?.done ? (latestRun.failed_tasks ?? 0) : 0;
  const live = Boolean(latestRun && !latestRun.done);

  // 趋势图数据：挂载时拉一次落盘运行历史，失败静默（图只是辅助）。
  const [historyRuns, setHistoryRuns] = useState<RunView[]>([]);
  useEffect(() => {
    getRuns()
      .then((result) => setHistoryRuns(result.runs ?? []))
      .catch(() => undefined);
  }, [latestRun?.id]);

  const statusLine = !latestRun
    ? { text: "还没有运行", cls: "text-ok", pulse: false }
    : latestRun.error
      ? { text: "运行失败", cls: "text-danger", pulse: false }
      : live
        ? { text: "运行中", cls: "text-brand", pulse: true }
        : failedTasks > 0
          ? { text: `完成 · ${failedTasks} 个任务失败`, cls: "text-warn", pulse: false }
          : { text: "执行成功", cls: "text-ok", pulse: false };
  const StatusIcon =
    statusLine.text === "运行失败" || failedTasks > 0 || statusLine.text === "运行中"
      ? statusLine.text === "运行中"
        ? Clock3
        : TriangleAlert
      : CheckCircle2;

  const metrics = [
    { label: "最近新增", value: latestRun?.saved_total ?? 0, hint: "个文件" },
    { label: "追更中", value: tracking, hint: `共 ${tasks.length} 个任务` },
    { label: "已完结", value: done, hint: "不再定期运行" },
    {
      label: "失效链接",
      value: bannedCount,
      hint: bannedCount > 0 ? "需要人工修复" : "全部链接有效",
      danger: bannedCount > 0,
    },
  ];

  return (
    <div className="space-y-4">
      <PageHeader
        title="总览"
        description="最近一次执行、追更进度与需要处理的链接。"
      />
      {bannedCount > 0 ? (
        <AlertStrip
          tone="warn"
          title={`${bannedCount} 条分享链接已失效`}
          description="受影响任务会跳过转存，直到链接被替换。"
          actionLabel="去处理"
          onAction={() => onGoTasks("banned")}
        />
      ) : (
        <AlertStrip
          tone="ok"
          title="所有任务运行正常"
          description={
            latestRun
              ? "最近一次运行没有发现失效链接。"
              : "还没有运行记录，保存任务后会按排程自动执行。"
          }
        />
      )}

      {/* 英雄卡：运行状态是主角，统计指标降为底部一行 */}
      <PageSection bodyClassName="p-0">
        <div className="px-4 pb-3 pt-4 sm:px-5">
          <div className="flex flex-wrap items-start justify-between gap-x-6 gap-y-4">
            <div className="min-w-0">
              <div className="flex items-center gap-2 text-[12px] font-medium text-muted-ink">
                最近运行
                {live ? (
                  <span className="inline-flex items-center gap-1 rounded-full bg-brand-soft px-1.5 py-0.5 text-[11px] text-brand-ink">
                    <span className="animate-pulse-live h-1.5 w-1.5 rounded-full bg-brand" />
                    运行中
                  </span>
                ) : null}
              </div>
              <div className="mt-1 flex items-baseline gap-2">
                <span className="text-[28px] font-semibold leading-none tabular-nums text-ink">
                  {seconds != null ? seconds.toFixed(1) : "—"}
                </span>
                <span className="text-[12px] text-subtle">秒</span>
                <span className="min-w-0 truncate text-[12px] text-muted-ink">
                  {latestRun ? latestRun.label : ""}
                </span>
              </div>
              <div
                className={cn(
                  "mt-1.5 flex items-center gap-1.5 text-[12px]",
                  statusLine.cls,
                )}
              >
                <StatusIcon className="h-3.5 w-3.5" />
                {statusLine.text}
              </div>
              {latestRun?.error ? (
                <div className="mt-2 max-w-[420px] rounded-md border border-danger/20 bg-danger-soft/40 px-2.5 py-2 text-[12px] text-danger">
                  {latestRun.error}
                </div>
              ) : null}
            </div>
            <div className="flex flex-col items-end gap-2">
              <div className="flex items-center gap-3">
                <button
                  onClick={onOpenLog}
                  className="inline-flex shrink-0 items-center gap-1 rounded px-1.5 py-0.5 text-[12px] font-medium text-muted-ink underline-offset-2 transition-colors hover:text-ink hover:underline"
                >
                  查看日志
                  <ArrowRight className="h-3 w-3" />
                </button>
                <button
                  onClick={onRunAll}
                  className="inline-flex shrink-0 items-center gap-1 rounded px-1.5 py-0.5 text-[12px] font-medium text-brand-ink underline-offset-2 transition-opacity duration-150 ease-out hover:underline active:opacity-70"
                >
                  <Play className="h-3 w-3" />
                  再跑一次
                </button>
              </div>
              <TrendBars runs={historyRuns} />
              <span className="text-[11px] text-subtle">近 7 天新增</span>
            </div>
          </div>
        </div>
        <div className="grid grid-cols-2 gap-px border-t border-line bg-line/40 sm:grid-cols-4">
          {metrics.map((metric) => (
            <div key={metric.label} className="bg-panel px-4 py-2.5">
              <div className="text-[12px] text-muted-ink">{metric.label}</div>
              <div className="mt-0.5 flex items-baseline gap-1.5">
                <span
                  className={cn(
                    "text-[16px] font-semibold tabular-nums",
                    metric.danger ? "text-danger" : "text-ink",
                  )}
                >
                  <AnimatedNumber value={metric.value} />
                </span>
                <span className="truncate text-[11px] text-subtle">
                  {metric.hint}
                </span>
              </div>
            </div>
          ))}
        </div>
      </PageSection>

      <PageSection
        title="最近任务"
        description={`配置中的前 5 个 · 共 ${tasks.length} 个`}
        action={
          <Button variant="ghost" size="sm" onClick={() => onGoTasks()}>
            查看全部
            <ArrowRight className="h-3.5 w-3.5" />
          </Button>
        }
        bodyClassName="p-0"
      >
        {recent.length === 0 ? (
          <div className="px-4 py-5 text-[12px] text-muted-ink">
            还没有任务，先用「新增任务」建立一个。
          </div>
        ) : (
          <TaskTable
            tasks={recent}
            onOpenTask={onOpenTask}
            onRun={onRunTask}
            onDelete={onDeleteTask}
            onRepair={onRepairTask}
            onToggle={onToggleTask}
          />
        )}
      </PageSection>

      {/* 需要处理 / 接下来 合并为一张卡，减少卡片海 */}
      <PageSection bodyClassName="p-0">
        <div className="grid lg:grid-cols-2 lg:divide-x lg:divide-line-soft">
          <div className="border-b border-line-soft lg:border-b-0">
            <div className="flex items-center justify-between px-4 pb-1 pt-3.5">
              <h2 className="text-[14px] font-semibold text-ink">需要处理</h2>
              {banned.length > 0 ? (
                <button
                  type="button"
                  onClick={() => onGoTasks("banned")}
                  className="inline-flex items-center gap-1 text-[12px] font-medium text-brand-ink underline-offset-2 hover:underline"
                >
                  全部 <ArrowRight className="h-3 w-3" />
                </button>
              ) : null}
            </div>
            {banned.length === 0 ? (
              <div className="flex items-center gap-2 px-4 py-4 text-[12px] text-muted-ink">
                <CheckCircle2 className="h-4 w-4 text-ok" />
                所有分享链接都有效。
              </div>
            ) : (
              <div className="divide-y divide-line-soft pb-1">
                {banned.map((task) => (
                  <div
                    key={task.index ?? task.taskname}
                    className="flex items-center gap-2 px-4 py-2.5"
                  >
                    <Wrench className="h-3.5 w-3.5 shrink-0 text-danger" />
                    <div className="min-w-0 flex-1">
                      <div className="truncate text-[12px] font-medium text-ink">
                        {task.taskname}
                      </div>
                      <div className="truncate text-[12px] text-danger">
                        {task.shareurl_ban}
                      </div>
                    </div>
                    <Button
                      variant="secondary"
                      size="sm"
                      className="border-danger/30 text-danger hover:bg-danger-soft hover:text-danger"
                      onClick={() => onRepairTask(task)}
                    >
                      修复
                    </Button>
                  </div>
                ))}
              </div>
            )}
          </div>
          <div>
            <div className="px-4 pb-1 pt-3.5">
              <h2 className="text-[14px] font-semibold text-ink">接下来</h2>
            </div>
            {upcoming.length === 0 ? (
              <div className="px-4 py-4 text-[12px] text-muted-ink">
                没有等待运行的任务。
              </div>
            ) : (
              <div className="divide-y divide-line-soft pb-1">
                {upcoming.map((task) => (
                  <button
                    key={task.index ?? task.taskname}
                    type="button"
                    onClick={() => onOpenTask(task)}
                    className="flex w-full items-center gap-2.5 px-4 py-2.5 text-left transition-colors hover:bg-line-soft/40"
                  >
                    <Clock3 className="h-3.5 w-3.5 shrink-0 text-subtle" />
                    <div className="min-w-0 flex-1">
                      <div className="truncate text-[12px] font-medium text-ink">
                        {task.taskname}
                      </div>
                      <div className="truncate font-mono text-[12px] text-muted-ink">
                        {task.next_run}
                      </div>
                    </div>
                    <TaskStatusBadge status={task.status} />
                  </button>
                ))}
              </div>
            )}
          </div>
        </div>
      </PageSection>

      <footer className="flex flex-col gap-1 pb-2 font-mono text-[12px] text-subtle sm:flex-row sm:items-center sm:justify-between">
        <span className="flex items-center gap-1.5">
          <CalendarClock className="h-3 w-3" />
          {tasks.length} 个订阅 · {tracking} 追更 ·{" "}
          {latestRun
            ? `最近运行 ${formatDateTime(latestRun.started_at)}`
            : "还没有运行记录"}
        </span>
        <span>quark-auto-save · self-hosted</span>
      </footer>
    </div>
  );
}
