import { Loader2 } from "lucide-react";
import type { Task } from "../lib/types";
import { Button } from "../components/ui/button";
import {
  Dialog,
  DialogBody,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "../components/ui/dialog";

export function DeleteTaskDialog({
  target,
  busy,
  onOpenChange,
  onConfirm,
}: {
  target: Task | null;
  busy: boolean;
  onOpenChange: (open: boolean) => void;
  onConfirm: () => void;
}) {
  return (
    <Dialog
      open={Boolean(target)}
      onOpenChange={(open) => {
        if (!open && !busy) onOpenChange(false);
      }}
    >
      <DialogContent className="w-[min(420px,calc(100vw-2rem))]">
        <DialogHeader>
          <DialogTitle>删除任务</DialogTitle>
          <DialogDescription>
            删除后不会移除已经转存的文件，只会停止后续追更。
          </DialogDescription>
        </DialogHeader>
        <DialogBody>
          <div className="rounded-md border border-danger/20 bg-danger-soft/40 px-3 py-2.5 text-[13px] text-ink">
            确定删除「{target?.taskname}」？
          </div>
        </DialogBody>
        <DialogFooter>
          <Button
            variant="secondary"
            size="sm"
            disabled={busy}
            onClick={() => onOpenChange(false)}
          >
            取消
          </Button>
          <Button
            variant="destructive"
            size="sm"
            disabled={busy || !target}
            onClick={onConfirm}
          >
            {busy ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : null}
            删除任务
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
