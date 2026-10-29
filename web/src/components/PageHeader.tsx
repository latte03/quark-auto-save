import type * as React from "react";
import { cn } from "../lib/utils";

export function PageHeader({
  title,
  description,
  actions,
  className,
}: {
  title: string;
  description?: string;
  actions?: React.ReactNode;
  className?: string;
}) {
  return (
    <div
      className={cn(
        "flex flex-wrap items-end justify-between gap-x-4 gap-y-2",
        className,
      )}
    >
      <div className="min-w-0">
        <h1 className="text-[20px] font-semibold tracking-[-0.02em] text-ink">
          {title}
        </h1>
        {description ? (
          <p className="mt-1 text-[13px] leading-5 text-muted-ink">
            {description}
          </p>
        ) : null}
      </div>
      {actions ? (
        <div className="flex shrink-0 items-center gap-2">{actions}</div>
      ) : null}
    </div>
  );
}
