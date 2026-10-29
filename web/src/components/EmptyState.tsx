import type { LucideIcon } from "lucide-react";
import { cn } from "../lib/utils";

export function EmptyState({
  icon: Icon,
  title,
  description,
  action,
  className,
}: {
  icon: LucideIcon;
  title: string;
  description?: string;
  action?: React.ReactNode;
  className?: string;
}) {
  return (
    <div
      className={cn(
        "flex flex-col items-center justify-center px-6 py-12 text-center",
        className,
      )}
    >
      <div className="grid h-10 w-10 place-items-center rounded-full bg-line-soft text-muted-ink">
        <Icon className="h-4 w-4" strokeWidth={1.6} />
      </div>
      <div className="mt-3 text-[13px] font-medium text-ink">{title}</div>
      {description ? (
        <p className="mt-1 max-w-[36ch] text-[12px] leading-5 text-muted-ink">
          {description}
        </p>
      ) : null}
      {action ? <div className="mt-3">{action}</div> : null}
    </div>
  );
}
