import {
  ArrowRight,
  CalendarClock,
  CheckCircle2,
  Clock3,
  ListChecks,
  Play,
  Settings2,
  Wrench,
} from "lucide-react";
import type { RunView, Task } from "../lib/types";
import { AlertStrip } from "../components/AlertStrip";
import { PageSection } from "../components/PageSection";
import { SummaryPanel, type Metric } from "../components/SummaryPanel";
import { TaskTable } from "../components/TaskTable";
import { TaskStatusBadge } from "../components/TaskStatusBadge";
import { Button } from "../components/ui/button";

type OverviewPageProps = {
  tasks: Task[];
  bannedCount: number;
  latestRun: RunView | null;
  onOpenTask: (task: Task) => void;
  onRunTask: (task: Task) => void;
  onDeleteTask: (task: Task) => void;
  onRepairTask: (task: Task) => void;
  onOpenLog: () => void;
  onRunAll: () => void;
  onGoTasks: (filter?: "banned") => void;
  onGoSettings: () => void;
};

function runSeconds(run: RunView | null) {
  if (!run?.finished_at) return null;
  const elapsed =
    (new Date(run.finished_at).getTime() - new Date(run.started_at).getTime()) / 1000;
  return Number.isFinite(elapsed) && elapsed >= 0 ? elapsed : null;
}

export function OverviewPage({
  tasks,
  bannedCount,
  latestRun,
  onOpenTask,
  onRunTask,
  onDeleteTask,
  onRepairTask,
  onOpenLog,
  onRunAll,
  onGoTasks,
  onGoSettings,
}: OverviewPageProps) {
  const banned = tasks.filter((task) => task.status === "banned");
  const upcoming = tasks
    .filter((task) => task.status === "tracking" || task.status === "waiting")
    .slice(0, 4);
  const recent = tasks.slice(0, 5);
  const tracking = tasks.filter((task) => task.status === "tracking").length;
  const done = tasks.filter((task) => task.status === "done").length;
  const seconds = runSeconds(latestRun);

  const metrics: Metric[] = [
    {
      label: "最近新增",
      value: latestRun?.saved_total ?? 0,
      unit: "FILES",
      hint: latestRun ? latestRun.label : "还没有运行记录",
      tone: "default",
    },
    {
      label: "追更中",
      value: tracking,
      unit: "TASKS",
      hint: `共 ${tasks.length} 个任务`,
      tone: "ok",
    },
    {
      label: "已完结",
      value: done,
      unit: "TASKS",
      hint: "不再定期运行",
      tone: "default",
    },
    {
      label: "失效链接",
      value: bannedCount,
      unit: "ALERT",
      hint: bannedCount > 0 ? "需要人工修复" : "全部链接有效",
      tone: "danger",
    },
  ];

  return (
    <div className="space-y-4">
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

      <div className="grid gap-4 xl:grid-cols-[1.65fr_1fr]">
        <SummaryPanel metrics={metrics} />
        <PageSection
          title="最近运行"
          description={
            latestRun
              ? `${latestRun.label} · ${new Date(latestRun.started_at).toLocaleString("zh-CN", { hour12: false })}`
              : "还没有运行记录"
          }
          action={
            <Button variant="ghost" size="sm" onClick={onOpenLog}>
              查看日志
              <ArrowRight className="h-3.5 w-3.5" />
            </Button>
          }
          bodyClassName="py-3"
        >
          <div className="flex items-start justify-between">
            <div>
              <div className="flex items-baseline gap-1.5">
                <span className="text-[26px] font-semibold leading-none tabular-nums text-ink">
                  {seconds != null ? seconds.toFixed(1) : "—"}
                </span>
                <span className="font-mono text-[12px] text-subtle">SECONDS</span>
              </div>
              <div
                className={`mt-1 flex items-center gap-1.5 text-[12px] ${
                  latestRun?.error
                    ? "text-danger"
                    : latestRun && !latestRun.done
                      ? "text-brand"
                      : "text-ok"
                }`}
              >
                <CheckCircle2 className="h-3.5 w-3.5" />
                {!latestRun
                  ? "还没有运行"
                  : latestRun.error
                    ? "运行失败"
                    : !latestRun.done
                      ? "运行中"
                      : "执行成功"}
              </div>
            </div>
            <Button variant="secondary" size="sm" onClick={onRunAll}>
              <Play className="h-3.5 w-3.5" />
              再跑一次
            </Button>
          </div>
          {latestRun?.error ? (
            <div className="mt-3 rounded-md border border-danger/20 bg-danger-soft/40 px-2.5 py-2 text-[12px] text-danger">
              {latestRun.error}
            </div>
          ) : null}
          <div className="mt-3 grid grid-cols-3 gap-2 border-t border-line pt-3">
            <div>
              <div className="text-[12px] text-muted">新增文件</div>
              <div className="mt-0.5 text-[14px] font-semibold tabular-nums">
                {latestRun?.saved_total ?? 0}
              </div>
            </div>
            <div>
              <div className="text-[12px] text-muted">有更新任务</div>
              <div className="mt-0.5 text-[14px] font-semibold tabular-nums">
                {latestRun?.tasks ? Object.keys(latestRun.tasks).length : 0}
              </div>
            </div>
            <div>
              <div className="text-[12px] text-muted">开始时间</div>
              <div className="mt-0.5 font-mono text-[12px] font-medium">
                {latestRun
                  ? new Date(latestRun.started_at).toLocaleTimeString("zh-CN", {
                      hour12: false,
                    })
                  : "—"}
              </div>
            </div>
          </div>
        </PageSection>
      </div>

      <PageSection
        title="最近任务"
        description="配置中的前 5 个任务"
        action={
          <Button variant="ghost" size="sm" onClick={() => onGoTasks()}>
            查看全部
            <ArrowRight className="h-3.5 w-3.5" />
          </Button>
        }
        bodyClassName="p-0"
      >
        <TaskTable
          tasks={recent}
          onOpenTask={onOpenTask}
          onRun={onRunTask}
          onDelete={onDeleteTask}
          onRepair={onRepairTask}
        />
      </PageSection>

      <div className="grid gap-4 lg:grid-cols-2 xl:grid-cols-3">
          <PageSection
            title="需要处理"
            description={banned.length > 0 ? "失效链接会一直被跳过" : "没有需要处理的问题"}
            action={
              banned.length > 0 ? (
                <Button variant="ghost" size="sm" onClick={() => onGoTasks("banned")}>
                  全部
                  <ArrowRight className="h-3.5 w-3.5" />
                </Button>
              ) : null
            }
            bodyClassName="p-0"
          >
            {banned.length === 0 ? (
              <div className="flex items-center gap-2 px-4 py-5 text-[12px] text-muted">
                <CheckCircle2 className="h-4 w-4 text-ok" />
                所有分享链接都有效。
              </div>
            ) : (
              <div className="divide-y divide-line-soft">
                {banned.map((task) => (
                  <div key={task.taskname} className="flex items-center gap-2 px-4 py-2.5">
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
          </PageSection>

          <PageSection
            title="接下来"
            description="按配置顺序"
            action={
              <Button variant="ghost" size="sm" onClick={() => onGoTasks()}>
                全部
                <ArrowRight className="h-3.5 w-3.5" />
              </Button>
            }
            bodyClassName="p-0"
          >
            <div className="divide-y divide-line-soft">
              {upcoming.map((task) => (
                <button
                  key={task.taskname}
                  onClick={() => onOpenTask(task)}
                  className="flex w-full items-center gap-2.5 px-4 py-2.5 text-left transition-colors hover:bg-line-soft/40"
                >
                  <Clock3 className="h-3.5 w-3.5 shrink-0 text-subtle" />
                  <div className="min-w-0 flex-1">
                    <div className="truncate text-[12px] font-medium text-ink">
                      {task.taskname}
                    </div>
                    <div className="truncate font-mono text-[12px] text-muted">
                      {task.next_run}
                    </div>
                  </div>
                  <TaskStatusBadge status={task.status} />
                </button>
              ))}
            </div>
          </PageSection>

          <PageSection title="快捷操作" bodyClassName="p-0">
            <div className="divide-y divide-line-soft">
              <button
                onClick={onRunAll}
                className="flex w-full items-center gap-2.5 px-4 py-2.5 text-left transition-colors hover:bg-line-soft/40"
              >
                <Play className="h-3.5 w-3.5 shrink-0 text-brand" />
                <span className="flex-1 text-[12px] font-medium text-ink">运行全部任务</span>
                <span className="font-mono text-[12px] text-subtle">RUN ALL</span>
              </button>
              <button
                onClick={() => onGoTasks()}
                className="flex w-full items-center gap-2.5 px-4 py-2.5 text-left transition-colors hover:bg-line-soft/40"
              >
                <ListChecks className="h-3.5 w-3.5 shrink-0 text-brand" />
                <span className="flex-1 text-[12px] font-medium text-ink">任务列表</span>
                <span className="font-mono text-[12px] text-subtle">TASKS</span>
              </button>
              <button
                onClick={onGoSettings}
                className="flex w-full items-center gap-2.5 px-4 py-2.5 text-left transition-colors hover:bg-line-soft/40"
              >
                <Settings2 className="h-3.5 w-3.5 shrink-0 text-brand" />
                <span className="flex-1 text-[12px] font-medium text-ink">插件设置</span>
                <span className="font-mono text-[12px] text-subtle">PLUGINS</span>
              </button>
            </div>
          </PageSection>
      </div>

      <footer className="flex flex-col gap-1 pb-2 font-mono text-[12px] text-subtle sm:flex-row sm:items-center sm:justify-between">
        <span className="flex items-center gap-1.5">
          <CalendarClock className="h-3 w-3" />
          {tasks.length} 个订阅 · {tracking} 追更 ·{" "}
          {latestRun
            ? `最近运行 ${new Date(latestRun.started_at).toLocaleString("zh-CN", { hour12: false })}`
            : "还没有运行记录"}
        </span>
        <span>quark-auto-save · self-hosted</span>
      </footer>
    </div>
  );
}
