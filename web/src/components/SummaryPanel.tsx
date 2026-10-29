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
        "overflow-hidden rounded-panel border border-line bg-panel",
        className,
      )}
      aria-label="运行概览"
    >
      <div className="flex items-center justify-between border-b border-line px-4 py-2.5">
        <div className="text-[14px] font-semibold text-ink">运行概览</div>
        <div className="font-mono text-[12px] tracking-[0.06em] text-subtle">
          实时
        </div>
      </div>
      <div className="grid grid-cols-2 divide-x divide-y divide-line lg:grid-cols-4 lg:divide-y-0">
        {metrics.map((metric) => (
          <div key={metric.label} className="min-w-0 px-4 py-3.5">
            <div className="flex items-center justify-between gap-2">
              <span className="truncate text-[12px] text-muted">{metric.label}</span>
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
            <div className="mt-1.5 text-[12px] text-muted">{metric.hint}</div>
          </div>
        ))}
      </div>
    </section>
  );
}
