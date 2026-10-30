import { cn } from "../lib/utils";

export type Metric = {
  label: string;
  value: number | string;
  unit?: string;
  hint: string;
  tone?: "default" | "ok" | "warn" | "danger";
};

const toneText = {
  default: "text-ink",
  ok: "text-ok",
  warn: "text-warn",
  danger: "text-danger",
};

export function SummaryPanel({
  metrics,
  className,
}: {
  metrics: Metric[];
  className?: string;
}) {
  return (
    <section
      className={cn(
        "flex h-full flex-col overflow-hidden rounded-panel bg-panel shadow-panel",
        className,
      )}
      aria-label="运行概览"
    >
      <div className="flex shrink-0 items-center justify-between px-4 pb-2.5 pt-3.5">
        <div className="text-[14px] font-semibold text-ink">运行概览</div>
        <div className="font-mono text-[12px] tracking-[0.06em] text-subtle">
          实时
        </div>
      </div>
      <div className="grid flex-1 grid-cols-2 gap-px border-t border-line/60 bg-line/50 lg:grid-cols-4">
        {metrics.map((metric) => (
          <div
            key={metric.label}
            className="flex min-w-0 flex-col bg-panel px-3.5 py-3"
          >
            <div className="flex items-center justify-between gap-2">
              <span className="truncate text-[12px] text-muted-ink">{metric.label}</span>
            </div>
            <div className="mt-1.5 flex items-baseline gap-1">
              <span
                className={cn(
                  "text-[28px] font-semibold leading-none tracking-[-0.02em] tabular-nums",
                  toneText[metric.tone ?? "default"],
                )}
              >
                {metric.value}
              </span>
              {metric.unit ? (
                <span className="font-mono text-[12px] text-subtle">
                  {metric.unit}
                </span>
              ) : null}
            </div>
            <div className="mt-auto pt-1.5 text-[12px] text-muted-ink">
              {metric.hint}
            </div>
          </div>
        ))}
      </div>
    </section>
  );
}
