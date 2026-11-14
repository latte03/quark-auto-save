import type { TaskStatus } from "../lib/types";
import { Badge } from "./ui/badge";

const statusMap: Record<
  TaskStatus,
  { label: string; variant: "ok" | "warn" | "danger" | "neutral" | "info" }
> = {
  tracking: { label: "追更中", variant: "ok" },
  // 排队/待命属于中性信息态，warn 留给真正需要注意的事。
  waiting: { label: "等待运行", variant: "info" },
  done: { label: "已完结", variant: "neutral" },
  banned: { label: "链接失效", variant: "danger" },
  once: { label: "待执行", variant: "info" },
  once_done: { label: "已完成", variant: "neutral" },
  paused: { label: "已停用", variant: "neutral" },
};

export function TaskStatusBadge({ status }: { status: TaskStatus }) {
  const item = statusMap[status];
  return (
    <Badge variant={item.variant} dot>
      {item.label}
    </Badge>
  );
}
