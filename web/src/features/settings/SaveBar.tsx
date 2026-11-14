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
  // 负 margin 抵消内容列 padding 与工具条 8px 内距的差值
  // （12/16/28px - 8px），左右边缘与顶部悬浮工具条对齐
  return (
    <div className="sticky bottom-2 z-10 -mx-1 flex items-center justify-between gap-3 rounded-xl border border-line bg-panel/90 px-3 py-2 shadow-panel backdrop-blur-md sm:-mx-2 lg:-mx-5">
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
