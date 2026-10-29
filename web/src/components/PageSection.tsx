import * as React from "react";
import { cn } from "../lib/utils";

export function PageSection({
  title,
  description,
  action,
  children,
  className,
  bodyClassName,
  id,
}: {
  title?: React.ReactNode;
  description?: React.ReactNode;
  action?: React.ReactNode;
  children: React.ReactNode;
  className?: string;
  bodyClassName?: string;
  id?: string;
}) {
  return (
    <section
      id={id}
      className={cn(
        "scroll-mt-16 overflow-hidden rounded-panel bg-panel shadow-panel",
        className,
      )}
    >
      {title ? (
        <div className="flex flex-col gap-2.5 px-4 pb-2.5 pt-3.5 sm:flex-row sm:items-start sm:justify-between sm:gap-4">
          <div className="min-w-0">
            <h2 className="text-[14px] font-semibold text-ink">{title}</h2>
            {description ? (
              <p className="mt-0.5 text-[12px] leading-4 text-muted-ink">
                {description}
              </p>
            ) : null}
          </div>
          {action ? (
            <div className="flex shrink-0 flex-wrap items-center gap-2">{action}</div>
          ) : null}
        </div>
      ) : null}
      <div className={cn("px-4 pb-3.5", title ? undefined : "pt-3.5", bodyClassName)}>
        {children}
      </div>
    </section>
  );
}
