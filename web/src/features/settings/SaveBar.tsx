import { Loader2, Save } from "lucide-react";
import { cn } from "../../lib/utils";
import { Button } from "../../components/ui/button";

export function SaveBar({
  dirty,
  saving,
  onSave,
}: {
  dirty: boolean;
  saving: boolean;
  onSave: () => void;
}) {
  return (
    <div className="-mb-4 sticky bottom-0 z-10 flex items-center justify-between gap-3 border-t border-line bg-canvas/95 py-2.5 backdrop-blur-sm lg:-mb-5">
      <span
        className={cn(
          "truncate text-[12px]",
          dirty ? "text-warn" : "text-muted-ink",
        )}
      >
        {dirty ? "有未保存的更改" : "保存后写入配置并重载调度"}
      </span>
      <Button size="sm" className="shrink-0" disabled={saving} onClick={onSave}>
        {saving ? (
          <Loader2 className="h-3.5 w-3.5 animate-spin" />
        ) : (
          <Save className="h-3.5 w-3.5" />
        )}
        {saving ? "保存中" : "保存配置"}
      </Button>
    </div>
  );
}
