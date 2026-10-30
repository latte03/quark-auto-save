import { useEffect, useState } from "react";
import { CheckCircle2, CircleAlert, Info, TriangleAlert, X } from "lucide-react";
import { cn } from "../lib/utils";

export type ToastTone = "info" | "ok" | "warn" | "error";

export type ToastData = {
  id: number;
  message: string;
  tone?: ToastTone;
};

const toneIcon = {
  info: Info,
  ok: CheckCircle2,
  warn: TriangleAlert,
  error: CircleAlert,
} as const;

const toneText = {
  info: "text-brand",
  ok: "text-ok",
  warn: "text-warn",
  error: "text-danger",
} as const;

const toneBorder = {
  info: "border-line",
  ok: "border-ok/25",
  warn: "border-warn/25",
  error: "border-danger/30",
} as const;

export function Toast({
  toast,
  onClose,
  className,
}: {
  toast: ToastData | null;
  onClose: () => void;
  className?: string;
}) {
  const [shown, setShown] = useState<ToastData | null>(toast);
  const [visible, setVisible] = useState(false);

  // Keep the last toast mounted for one exit transition instead of unmounting
  // the moment the parent clears it.
  useEffect(() => {
    if (toast) {
      setShown(toast);
      const frame = requestAnimationFrame(() => setVisible(true));
      return () => cancelAnimationFrame(frame);
    }
    setVisible(false);
    const timer = window.setTimeout(() => setShown(null), 180);
    return () => window.clearTimeout(timer);
  }, [toast]);

  if (!shown) return null;

  const tone = shown.tone ?? "info";
  const Icon = toneIcon[tone];

  return (
    <div
      className={cn(
        "fixed right-4 bottom-4 z-[80] flex w-fit max-w-[360px] items-start gap-2.5 rounded-panel border bg-panel/95 px-3.5 py-2.5 shadow-lg backdrop-blur-md transition-[opacity,translate,scale]",
        visible
          ? "translate-y-0 scale-100 opacity-100 duration-200 ease-out"
          : "pointer-events-none translate-y-2 scale-[0.98] opacity-0 duration-150 ease-out",
        toneBorder[tone],
        className,
      )}
      role={tone === "error" ? "alert" : "status"}
    >
      <Icon className={cn("mt-0.5 h-4 w-4 shrink-0", toneText[tone])} />
      <span className="text-[12px] leading-5 text-ink">{shown.message}</span>
      <button
        type="button"
        onClick={onClose}
        aria-label="关闭提示"
        className="-mr-0.5 ml-0.5 grid h-5 w-5 shrink-0 place-items-center rounded text-subtle transition-colors duration-150 ease-out hover:bg-line-soft hover:text-ink"
      >
        <X className="h-3 w-3" />
      </button>
    </div>
  );
}
