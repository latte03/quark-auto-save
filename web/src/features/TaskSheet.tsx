import { useState } from "react";
import {
  FlaskConical,
  Loader2,
  Play,
  Trash2,
} from "lucide-react";
import type { AppConfig, PluginConfig, Task } from "../lib/types";
import { Button } from "../components/ui/button";
import {
  Sheet,
  SheetBody,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from "../components/ui/sheet";
import { TaskStatusBadge } from "../components/TaskStatusBadge";
import { ConfirmDialog } from "../components/ConfirmDialog";
import { useToast } from "../components/ToastProvider";
import { FilePickerDialog } from "./FilePickerDialog";
import { SearchPreviewDialog } from "./SearchPreviewDialog";
import { BasicInfoSection } from "./task-sheet/BasicInfoSection";
import { ScheduleSection } from "./task-sheet/ScheduleSection";
import { MatchSection } from "./task-sheet/MatchSection";
import { PluginsSection } from "./task-sheet/PluginsSection";
import {
  TaskDraftProvider,
  type PickerMode,
} from "./task-sheet/TaskDraftContext";
import { useTaskDraft } from "./task-sheet/useTaskDraft";
import { useShareSearch } from "./task-sheet/useShareSearch";
import { useLinkValidation } from "./task-sheet/useLinkValidation";

type TaskSheetProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  task: Task | null;
  magicRegex: AppConfig["magic_regex"];
  pluginDefaults: Record<string, PluginConfig>;
  searchReady: boolean;
  onOpenSettings: () => void;
  onSave: (task: Task, runAfter: boolean) => void;
  onTry: (task: Task) => void;
  onDelete: (task: Task) => void;
};

export function TaskSheet({
  open,
  onOpenChange,
  task,
  magicRegex,
  pluginDefaults,
  searchReady,
  onOpenSettings,
  onSave,
  onTry,
  onDelete,
}: TaskSheetProps) {
  const notify = useToast();
  const { draft, setDraft, dirty, update, updateAddition } = useTaskDraft({
    open,
    task,
    magicRegex,
    pluginDefaults,
  });
  const search = useShareSearch(open, searchReady);
  const link = useLinkValidation(open, task);
  const [picker, setPicker] = useState<PickerMode | null>(null);
  const [confirmClose, setConfirmClose] = useState(false);

  const isNew = !task;
  const banned = Boolean(draft.shareurl_ban);
  const onceMode = draft.mode === "once";

  const requestClose = () => {
    if (!dirty) {
      onOpenChange(false);
      return;
    }
    setConfirmClose(true);
  };

  const submit = (runAfter: boolean) => {
    if (!draft.taskname.trim() || !draft.shareurl.trim() || !draft.savepath.trim()) {
      notify("任务名称、分享链接、保存路径为必填项", "warn");
      return;
    }
    const next: Task = {
      ...draft,
      shareurl_ban: undefined,
      status: banned ? "tracking" : draft.status,
    };
    if (!onceMode) delete next.mode;
    onSave(next, runAfter);
    onOpenChange(false);
  };

  const tryRun = () => {
    if (!draft.taskname.trim() || !draft.shareurl.trim() || !draft.savepath.trim()) {
      notify("任务名称、分享链接、保存路径为必填项", "warn");
      return;
    }
    onTry({
      ...draft,
      shareurl_ban: undefined,
      status: banned ? "tracking" : draft.status,
    });
    notify("试运行已开始，可点「运行记录」查看日志", "ok");
  };

  return (
    <TaskDraftProvider
      value={{
        draft,
        setDraft,
        update,
        updateAddition,
        magicRegex,
        pluginDefaults,
        search,
        link,
        searchReady,
        onOpenSettings,
        openPicker: setPicker,
      }}
    >
      <Sheet
        open={open}
        onOpenChange={(next) => {
          if (next) onOpenChange(true);
          else requestClose();
        }}
      >
        <SheetContent className="w-full sm:!max-w-[600px]">
          <SheetHeader>
            <div className="flex items-center gap-2">
              <SheetTitle>{isNew ? "新增任务" : draft.taskname}</SheetTitle>
              {isNew ? null : <TaskStatusBadge status={draft.status} />}
            </div>
            <SheetDescription>
              {isNew
                ? "填写分享链接与保存规则；保存后可立即运行。"
                : "编辑任务配置，或修复失效链接后立即重跑。"}
            </SheetDescription>
          </SheetHeader>

          <SheetBody className="space-y-4 [&_[data-slot=field-label]]:text-muted-ink">
            <BasicInfoSection />
            <ScheduleSection open={open} />
            <MatchSection open={open} />
            <PluginsSection open={open} />
          </SheetBody>

          <SheetFooter className="flex-row items-center justify-between gap-2">
            {isNew ? (
              <span className="text-[12px] text-subtle">
                保存后可在任务列表继续编辑
              </span>
            ) : (
              <Button
                variant="ghost"
                size="sm"
                className="text-danger hover:bg-danger-soft hover:text-danger"
                onClick={() => {
                  onDelete(draft);
                  onOpenChange(false);
                }}
              >
                <Trash2 className="h-3.5 w-3.5" />
                删除任务
              </Button>
            )}
            <div className="flex items-center gap-2">
              <Button variant="secondary" size="sm" onClick={requestClose}>
                取消
              </Button>
              <Button variant="secondary" size="sm" onClick={tryRun}>
                <FlaskConical className="h-3.5 w-3.5" />
                试运行
              </Button>
              <Button variant="secondary" size="sm" onClick={() => submit(false)}>
                保存
              </Button>
              <Button size="sm" onClick={() => submit(true)}>
                {banned ? <Loader2 className="h-3.5 w-3.5" /> : <Play className="h-3.5 w-3.5" />}
                {onceMode ? "保存并执行" : "保存并运行"}
              </Button>
            </div>
          </SheetFooter>
        </SheetContent>
      </Sheet>

      <ConfirmDialog
        open={confirmClose}
        onOpenChange={setConfirmClose}
        title="放弃未保存的修改？"
        description={
          isNew
            ? "这个新任务还没有保存，关闭后填写的内容会丢失。"
            : "关闭后，这个任务里改动的内容不会保存。"
        }
        confirmLabel="放弃修改"
        cancelLabel="继续编辑"
        onConfirm={() => {
          setConfirmClose(false);
          onOpenChange(false);
        }}
      />

      <FilePickerDialog
        open={picker !== null}
        mode={picker ?? "savepath"}
        task={draft}
        onOpenChange={(next) => {
          if (!next) setPicker(null);
        }}
        onPickPath={(path) => {
          update("savepath", path);
          setPicker(null);
          notify(`保存路径已设为 ${path}`, "ok");
        }}
      />

      <SearchPreviewDialog
        open={search.previewHit !== null}
        hit={search.previewHit}
        task={draft}
        onOpenChange={(next) => {
          if (!next) search.setPreviewHit(null);
        }}
        onUse={(hit) => {
          setDraft((prev) => ({
            ...prev,
            shareurl: hit.shareurl,
            shareurl_ban: undefined,
            status: "tracking",
          }));
          link.markOk();
          search.setPreviewHit(null);
          search.setShow(false);
          notify("已填入候选链接，保存后生效", "ok");
        }}
      />
    </TaskDraftProvider>
  );
}
