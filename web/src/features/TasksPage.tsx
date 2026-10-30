import { useEffect, useMemo, useState } from "react";
import { ListFilter, Search } from "lucide-react";
import type { RunView, Task } from "../lib/types";
import { cn } from "../lib/utils";
import { AlertStrip } from "../components/AlertStrip";
import { EmptyState } from "../components/EmptyState";
import { PageHeader } from "../components/PageHeader";
import { TaskTable } from "../components/TaskTable";
import { Button } from "../components/ui/button";
import { Input } from "../components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "../components/ui/select";

export type TaskFilter = "all" | "tracking" | "waiting" | "done" | "banned";
type Sort = "recent" | "name" | "next" | "status";

const filterLabels: Record<TaskFilter, string> = {
  all: "全部",
  tracking: "追更中",
  waiting: "等待运行",
  done: "已完结",
  banned: "失效",
};

const statusOrder: Record<Task["status"], number> = {
  banned: 0,
  once: 1,
  waiting: 2,
  tracking: 3,
  done: 4,
  once_done: 5,
};

// One-shot tasks count as "waiting" until they run and as "done" afterwards.
function inFilter(status: Task["status"], filter: TaskFilter): boolean {
  switch (filter) {
    case "all":
      return true;
    case "waiting":
      return status === "waiting" || status === "once";
    case "done":
      return status === "done" || status === "once_done";
    default:
      return status === filter;
  }
}

type TasksPageProps = {
  tasks: Task[];
  initialFilter: TaskFilter;
  query: string;
  latestRun: RunView | null;
  onQueryChange: (value: string) => void;
  onOpenTask: (task: Task) => void;
  onRunTask: (task: Task) => void;
  onDeleteTask: (task: Task) => void;
  onRepairTask: (task: Task) => void;
};

export function TasksPage({
  tasks,
  initialFilter,
  query,
  latestRun,
  onQueryChange,
  onOpenTask,
  onRunTask,
  onDeleteTask,
  onRepairTask,
}: TasksPageProps) {
  const [filter, setFilter] = useState<TaskFilter>(initialFilter);
  const [sort, setSort] = useState<Sort>("recent");
  const bannedCount = tasks.filter((task) => task.status === "banned").length;

  useEffect(() => {
    setFilter(initialFilter);
  }, [initialFilter]);

  const filtered = useMemo(() => {
    const keyword = query.trim().toLowerCase();
    const list = tasks.filter((task) => {
      if (!inFilter(task.status, filter)) return false;
      if (!keyword) return true;
      return (
        task.taskname.toLowerCase().includes(keyword) ||
        task.savepath.toLowerCase().includes(keyword) ||
        task.shareurl.toLowerCase().includes(keyword)
      );
    });
    if (sort === "name") {
      return [...list].sort((a, b) => a.taskname.localeCompare(b.taskname, "zh"));
    }
    if (sort === "next") {
      return [...list].sort((a, b) => a.next_run.localeCompare(b.next_run, "zh"));
    }
    if (sort === "status") {
      return [...list].sort(
        (a, b) => statusOrder[a.status] - statusOrder[b.status],
      );
    }
    return list;
  }, [tasks, filter, query, sort]);

  const counts: Record<TaskFilter, number> = {
    all: tasks.length,
    tracking: tasks.filter((task) => task.status === "tracking").length,
    waiting: tasks.filter((task) => inFilter(task.status, "waiting")).length,
    done: tasks.filter((task) => inFilter(task.status, "done")).length,
    banned: bannedCount,
  };

  return (
    <div className="space-y-4">
      <PageHeader
        title="任务"
        description="每个任务对应一个分享链接，按排程追更或只执行一次。"
      />
      {bannedCount > 0 ? (
        <AlertStrip
          tone="warn"
          title={`${bannedCount} 条分享链接已失效`}
          description="这些任务会被跳过，直到链接被替换。"
          actionLabel="只看失效"
          onAction={() => setFilter("banned")}
        />
      ) : null}

      <section className="overflow-hidden rounded-panel bg-panel shadow-panel">
        <div className="flex flex-wrap items-center gap-2 px-3 py-2.5">
          <div className="flex flex-wrap items-center gap-1">
            {(Object.keys(filterLabels) as TaskFilter[]).map((key) => (
              <button
                key={key}
                onClick={() => setFilter(key)}
                className={cn(
                  "flex h-7 items-center gap-1.5 rounded-md px-2 text-[12px] transition-[background-color,color] duration-150 ease-out",
                  filter === key
                    ? "bg-brand-soft font-medium text-brand-ink"
                    : "text-muted-ink hover:bg-line-soft hover:text-ink",
                )}
              >
                {filterLabels[key]}
                <span className="font-mono text-[12px] tabular-nums text-subtle">
                  {counts[key]}
                </span>
              </button>
            ))}
          </div>
          <div className="ml-auto flex items-center gap-2">
            <div className="relative md:hidden">
              <Search className="pointer-events-none absolute left-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-subtle" />
              <Input
                id="task-filter-search"
                name="task-filter-search"
                type="search"
                autoComplete="off"
                value={query}
                onChange={(event) => onQueryChange(event.target.value)}
                placeholder="筛选名称或路径"
                aria-label="筛选任务"
                className="h-7 w-[190px] pl-8 text-[12px]"
              />
            </div>
            <Select
              value={sort}
              items={[
                { value: "recent", label: "默认顺序" },
                { value: "name", label: "名称" },
                { value: "next", label: "下次运行" },
                { value: "status", label: "状态" },
              ]}
              onValueChange={(value) => value && setSort(value as Sort)}
            >
              <SelectTrigger className="h-7 w-[118px] text-[12px]" aria-label="排序">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="recent">默认顺序</SelectItem>
                <SelectItem value="name">名称</SelectItem>
                <SelectItem value="next">下次运行</SelectItem>
                <SelectItem value="status">状态</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </div>

        {filtered.length === 0 ? (
          <EmptyState
            icon={ListFilter}
            title="没有匹配的任务"
            description="换一个筛选条件，或清除搜索关键词。"
            action={
              <Button
                variant="secondary"
                size="sm"
                onClick={() => {
                  setFilter("all");
                  onQueryChange("");
                }}
              >
                清除筛选
              </Button>
            }
          />
        ) : (
          <TaskTable
            tasks={filtered}
            onOpenTask={onOpenTask}
            onRun={onRunTask}
            onDelete={onDeleteTask}
            onRepair={onRepairTask}
          />
        )}

        <div className="px-3 py-2 font-mono text-[12px] text-subtle">
          显示 {filtered.length} / {tasks.length} 个任务
        </div>
      </section>

      <footer className="flex flex-col gap-1 pb-2 font-mono text-[12px] text-subtle sm:flex-row sm:items-center sm:justify-between">
        <span>
          {tasks.length} 个订阅 · {counts.tracking} 追更 ·{" "}
          {latestRun
            ? `最近运行 ${new Date(latestRun.started_at).toLocaleString("zh-CN", { hour12: false })}`
            : "还没有运行记录"}
        </span>
        <span>quark-auto-save · self-hosted</span>
      </footer>
    </div>
  );
}
