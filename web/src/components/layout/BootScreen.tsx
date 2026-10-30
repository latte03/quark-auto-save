import { CircleAlert, Loader2 } from "lucide-react";
import { Button } from "../ui/button";

export function BootScreen({
  error,
  onRetry,
}: {
  error: string;
  onRetry: () => void;
}) {
  if (error) {
    return (
      <div className="grid min-h-screen place-items-center bg-canvas px-4">
        <div
          role="alert"
          className="flex w-full max-w-[420px] flex-col items-start gap-3 rounded-panel border border-danger/25 bg-panel px-4 py-3.5 shadow-panel"
        >
          <div className="flex items-center gap-2 text-[13px] font-medium text-ink">
            <CircleAlert className="h-4 w-4 text-danger" />
            无法连接控制台
          </div>
          <p className="text-[12px] leading-5 text-muted-ink">{error}</p>
          <Button size="sm" variant="secondary" onClick={onRetry}>
            重试
          </Button>
        </div>
      </div>
    );
  }
  return (
    <div className="grid min-h-screen place-items-center bg-canvas px-4">
      <div className="flex items-center gap-2 text-[12px] text-muted-ink">
        <Loader2 className="h-4 w-4 animate-spin" />
        正在连接控制台…
      </div>
    </div>
  );
}
