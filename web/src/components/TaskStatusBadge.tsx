import type { TaskStatus } from "../lib/types";
import { Badge } from "./ui/badge";

const statusMap: Record<
  TaskStatus,
  { label: string; variant: "ok" | "warn" | "danger" | "neutral" }
> = {
  tracking: { label: "追更中", variant: "ok" },
  waiting: { label: "等待运行", variant: "warn" },
  done: { label: "已完结", variant: "neutral" },
  banned: { label: "链接失效", variant: "danger" },
};

export function TaskStatusBadge({ status }: { status: TaskStatus }) {
  const item = statusMap[status];
  return (
    <Badge variant={item.variant} dot>
      {item.label}
    </Badge>
  );
}
