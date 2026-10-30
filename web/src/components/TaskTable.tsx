import {
  MoreHorizontal,
  Pencil,
  Play,
  Trash2,
  Wrench,
} from "lucide-react";
import type { Task } from "../lib/types";
import { cn } from "../lib/utils";
import { TaskStatusBadge } from "./TaskStatusBadge";
import { Button } from "./ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "./ui/dropdown-menu";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "./ui/tooltip";

type TaskTableProps = {
  tasks: Task[];
  onOpenTask: (task: Task) => void;
  onRun: (task: Task) => void;
  onDelete: (task: Task) => void;
  onRepair: (task: Task) => void;
  className?: string;
};

export function TaskTable({
  tasks,
  onOpenTask,
  onRun,
  onDelete,
  onRepair,
  className,
}: TaskTableProps) {
  return (
    <div className={cn("overflow-x-auto", className)}>
      <table className="w-full min-w-[860px] border-collapse text-left" aria-label="任务列表">
        <thead>
          <tr className="border-b border-line bg-line-soft/35">
            <th className="h-8 whitespace-nowrap px-3 text-[12px] font-medium text-muted-ink">
              任务
            </th>
            <th className="h-8 whitespace-nowrap px-3 text-[12px] font-medium text-muted-ink">
              保存路径
            </th>
            <th className="h-8 whitespace-nowrap px-3 text-[12px] font-medium text-muted-ink">
              最新
            </th>
            <th className="h-8 whitespace-nowrap px-3 text-[12px] font-medium text-muted-ink">
              下次运行
            </th>
            <th className="h-8 whitespace-nowrap px-3 text-[12px] font-medium text-muted-ink">
              状态
            </th>
            <th className="h-8 whitespace-nowrap px-3 text-right text-[12px] font-medium text-muted-ink">
              操作
            </th>
          </tr>
        </thead>
        <tbody>
          {tasks.map((task, index) => {
            const progress =
              task.progress.expected > 0
                ? Math.min(100, (task.progress.saved / task.progress.expected) * 100)
                : 0;
            return (
              <tr
                key={`${task.taskname}-${index}`}
                className={cn(
                  "group cursor-pointer border-b border-line-soft outline-none transition-colors duration-150 ease-out last:border-b-0 hover:bg-brand-soft/30 focus-visible:bg-brand-soft/40 active:bg-brand-soft/45",
                  task.status === "banned" && "bg-danger-soft/30 hover:bg-danger-soft/50",
                )}
                tabIndex={0}
                onClick={() => onOpenTask(task)}
                onKeyDown={(event) => {
                  // 行本身可聚焦只是给键盘留一条打开任务的路；空格留给页面滚动。
                  if (event.target !== event.currentTarget) return;
                  if (event.key === "Enter") {
                    event.preventDefault();
                    onOpenTask(task);
                  }
                }}
              >
                <td className="min-w-[160px] px-3 py-2.5">
                  <div className="flex min-w-0 flex-col">
                    <div className="flex max-w-[220px] items-center gap-1.5">
                      <span className="truncate text-[14px] font-medium text-ink">
                        {task.taskname}
                      </span>
                      {task.mode === "once" ? (
                        <span className="shrink-0 rounded border border-line px-1 text-[11px] leading-4 text-subtle">
                          单次
                        </span>
                      ) : null}
                      {task.shareurl_ban ? (
                        <Wrench className="h-3 w-3 shrink-0 text-danger" />
                      ) : null}
                    </div>
                    <div className="mt-0.5 max-w-[220px] truncate font-mono text-[13px] text-subtle">
                      {task.magic_name || task.pattern}
                    </div>
                  </div>
                </td>
                <td className="px-3 py-2.5">
                  <div className="w-[180px] truncate font-mono text-[13px] text-muted-ink 2xl:w-[260px]">
                    {task.savepath}
                  </div>
                </td>
                <td className="whitespace-nowrap px-3 py-2.5">
                  <div className="flex items-center gap-2">
                    <span className="font-mono text-[12px] tabular-nums text-ink">
                      {task.latest}
                    </span>
                    {task.progress.expected > 0 ? (
                      <div className="flex items-center gap-1.5">
                        <div
                          role="progressbar"
                          aria-label={`${task.taskname} 转存进度`}
                          aria-valuemin={0}
                          aria-valuemax={task.progress.expected}
                          aria-valuenow={task.progress.saved}
                          className="h-1 w-12 overflow-hidden rounded-full bg-line"
                        >
                          <div
                            className={cn(
                              "h-full origin-left rounded-full transition-transform duration-500 ease-out",
                              task.status === "banned"
                                ? "bg-danger"
                                : task.status === "done" || task.status === "once_done"
                                  ? "bg-muted"
                                  : "bg-ok",
                            )}
                            style={{ transform: `scaleX(${progress / 100})` }}
                          />
                        </div>
                        <span className="font-mono text-[12px] text-subtle">
                          {task.progress.saved}/{task.progress.expected}
                        </span>
                      </div>
                    ) : null}
                  </div>
                </td>
                <td className="whitespace-nowrap px-3 py-2.5">
                  <span className="font-mono text-[12px] text-muted-ink">{task.next_run}</span>
                </td>
                <td className="whitespace-nowrap px-3 py-2.5">
                  <TaskStatusBadge status={task.status} />
                </td>
                <td className="px-3 py-2.5">
                  <div className="flex items-center justify-end gap-0.5">
                    {task.status === "banned" ? (
                      <Button
                        variant="secondary"
                        size="sm"
                        className="h-7 border-danger/30 px-2 text-[12px] text-danger hover:bg-danger-soft hover:text-danger"
                        onClick={(event) => {
                          event.stopPropagation();
                          onRepair(task);
                        }}
                      >
                        修复
                      </Button>
                    ) : (
                      <>
                        <Tooltip>
                          <TooltipTrigger
                            render={
                              <Button
                                variant="ghost"
                                size="icon-sm"
                                aria-label={`运行 ${task.taskname}`}
                                onClick={(event) => {
                                  event.stopPropagation();
                                  onRun(task);
                                }}
                              >
                                <Play className="h-3.5 w-3.5" />
                              </Button>
                            }
                          />
                          <TooltipContent>运行</TooltipContent>
                        </Tooltip>
                        <Tooltip>
                          <TooltipTrigger
                            render={
                              <Button
                                variant="ghost"
                                size="icon-sm"
                                aria-label={`编辑 ${task.taskname}`}
                                onClick={(event) => {
                                  event.stopPropagation();
                                  onOpenTask(task);
                                }}
                              >
                                <Pencil className="h-3.5 w-3.5" />
                              </Button>
                            }
                          />
                          <TooltipContent>编辑</TooltipContent>
                        </Tooltip>
                      </>
                    )}
                    <DropdownMenu>
                      <DropdownMenuTrigger
                        render={
                          <Button
                            variant="ghost"
                            size="icon-sm"
                            aria-label={`更多操作 ${task.taskname}`}
                            onClick={(event) => event.stopPropagation()}
                          >
                            <MoreHorizontal className="h-3.5 w-3.5" />
                          </Button>
                        }
                      />
                      <DropdownMenuContent align="end">
                        <DropdownMenuItem
                          onClick={(event) => {
                            event.stopPropagation();
                            onOpenTask(task);
                          }}
                        >
                          <Pencil className="h-3.5 w-3.5" />
                          编辑任务
                        </DropdownMenuItem>
                        <DropdownMenuItem
                          onClick={(event) => {
                            event.stopPropagation();
                            onRun(task);
                          }}
                        >
                          <Play className="h-3.5 w-3.5" />
                          立即运行
                        </DropdownMenuItem>
                        <DropdownMenuSeparator />
                        <DropdownMenuItem
                          className="text-danger focus:bg-danger-soft focus:text-danger"
                          onClick={(event) => {
                            event.stopPropagation();
                            onDelete(task);
                          }}
                        >
                          <Trash2 className="h-3.5 w-3.5" />
                          删除任务
                        </DropdownMenuItem>
                      </DropdownMenuContent>
                    </DropdownMenu>
                  </div>
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
