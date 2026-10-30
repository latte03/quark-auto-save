import { AlertTriangle, ArrowRight, CheckCircle2 } from "lucide-react";
import { cn } from "../lib/utils";

type AlertStripProps = {
  tone?: "warn" | "danger" | "ok";
  title: string;
  description?: string;
  actionLabel?: string;
  onAction?: () => void;
  className?: string;
};

export function AlertStrip({
  tone = "warn",
  title,
  description,
  actionLabel,
  onAction,
  className,
}: AlertStripProps) {
  const styles = {
    warn: "bg-warn-soft text-warn",
    danger: "bg-danger-soft text-danger",
    ok: "bg-ok-soft text-ok",
  }[tone];
  const Icon = tone === "ok" ? CheckCircle2 : AlertTriangle;

  return (
    <div
      className={cn(
        "flex flex-wrap items-center gap-x-2.5 gap-y-1 rounded-panel px-3.5 py-2.5",
        styles,
        className,
      )}
      role="status"
    >
      <Icon className="h-4 w-4 shrink-0" strokeWidth={1.8} />
      <div className="min-w-0 flex-1">
        <span className="text-[13px] font-semibold">{title}</span>
        {description ? (
          <span className="block text-[13px] sm:ml-2 sm:inline">{description}</span>
        ) : null}
      </div>
      {actionLabel ? (
        <button
          type="button"
          onClick={onAction}
          className="inline-flex shrink-0 items-center gap-1 rounded px-1.5 py-0.5 text-[12px] font-medium whitespace-nowrap underline-offset-2 transition-opacity duration-150 ease-out hover:underline active:opacity-70"
        >
          {actionLabel}
          <ArrowRight className="h-3 w-3" />
        </button>
      ) : null}
    </div>
  );
}
