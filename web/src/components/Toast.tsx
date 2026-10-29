import { CheckCircle2, CircleAlert, Info, TriangleAlert, X } from "lucide-react";
import { cn } from "../lib/utils";

export type ToastTone = "info" | "ok" | "warn" | "error";

export type ToastData = {
  id: number;
  message: string;
  tone?: ToastTone;
};

export function Toast({
  toast,
  onClose,
  className,
}: {
  toast: ToastData | null;
  onClose: () => void;
  className?: string;
}) {
  if (!toast) return null;
  const Icon =
    toast.tone === "ok"
      ? CheckCircle2
      : toast.tone === "error"
        ? CircleAlert
        : toast.tone === "warn"
          ? TriangleAlert
          : Info;
  return (
    <div
      className={cn(
        "pop-in fixed bottom-4 right-4 z-[80] flex max-w-[360px] items-start gap-2 rounded-panel border bg-panel px-3 py-2.5 shadow-lg",
        toast.tone === "ok"
          ? "border-ok/25"
          : toast.tone === "error"
            ? "border-danger/30"
            : toast.tone === "warn"
              ? "border-warn/25"
              : "border-line",
        className,
      )}
      role={toast.tone === "error" ? "alert" : "status"}
    >
      <Icon
        className={cn(
          "mt-0.5 h-4 w-4 shrink-0",
          toast.tone === "ok"
            ? "text-ok"
            : toast.tone === "error"
              ? "text-danger"
              : toast.tone === "warn"
                ? "text-warn"
                : "text-brand",
        )}
      />
      <span className="text-[12px] leading-5 text-ink">{toast.message}</span>
      <button
        onClick={onClose}
        aria-label="关闭提示"
        className="ml-1 grid h-5 w-5 shrink-0 place-items-center rounded text-subtle hover:bg-line-soft hover:text-ink"
      >
        <X className="h-3 w-3" />
      </button>
    </div>
  );
}
