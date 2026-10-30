import { useEffect, useId, useState } from "react";
import { ChevronDown, Sparkles } from "lucide-react";
import { previewRename } from "../../lib/api";
import { cn } from "../../lib/utils";
import { Button } from "../../components/ui/button";
import { Field, FieldLabel } from "../../components/ui/field";
import { Input } from "../../components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "../../components/ui/select";
import { SwitchField } from "../../components/SwitchField";
import { sectionTitle } from "./constants";
import { useTaskDraftContext } from "./TaskDraftContext";

export function MatchSection({ open }: { open: boolean }) {
  const { draft, setDraft, update, magicRegex, openPicker } =
    useTaskDraftContext();
  const [showRules, setShowRules] = useState(false);
  const [samplePreview, setSamplePreview] = useState("");
  const selectId = useId();
  const replaceId = useId();

  useEffect(() => {
    if (open) setShowRules(false);
  }, [open]);

  useEffect(() => {
    if (!open) return;
    if (!draft.magic_id && !draft.pattern) {
      setSamplePreview("不筛选，保持原名");
      return;
    }
    let cancelled = false;
    const timer = window.setTimeout(() => {
      previewRename({
        taskname: draft.taskname,
        magic_id: draft.magic_id,
        pattern: draft.pattern,
        replace: draft.replace,
        files: ["Black.Mirror.S07E05.mkv"],
      })
        .then((result) => {
          if (cancelled) return;
          const file = result.files[0];
          setSamplePreview(file?.matched ? file.renamed : "样本不匹配，跳过");
        })
        .catch(() => {
          if (!cancelled) setSamplePreview("预览失败");
        });
    }, 250);
    return () => {
      cancelled = true;
      window.clearTimeout(timer);
    };
  }, [open, draft.magic_id, draft.pattern, draft.replace, draft.taskname]);

  const ruleName = draft.magic_id
    ? (magicRegex.find((rule) => rule.id === draft.magic_id)?.name ?? "自定义规则")
    : "";
  const ruleSummary =
    (ruleName || draft.pattern || "不筛选，保持原名") +
    (draft.replace ? ` → ${draft.replace}` : "");

  return (
    <section className="space-y-3 border-t border-line pt-3.5">
      <div className="flex items-center justify-between gap-2">
        <h3 className={sectionTitle}>
          <button
            type="button"
            aria-expanded={showRules}
            onClick={() => setShowRules((prev) => !prev)}
            className="flex min-w-0 items-center gap-1.5 text-left"
          >
            保存规则
            <ChevronDown
              className={cn(
                "h-3.5 w-3.5 shrink-0 text-subtle transition-transform",
                showRules && "rotate-180",
              )}
            />
          </button>
        </h3>
        <div className="flex min-w-0 items-center gap-1.5">
          {showRules ? null : (
            <span className="max-w-[220px] truncate font-mono text-[11px] text-subtle">
              {ruleSummary}
            </span>
          )}
          <Button
            variant="ghost"
            size="sm"
            onClick={() => openPicker("share")}
          >
            <Sparkles className="h-3.5 w-3.5" />
            正则预览
          </Button>
        </div>
      </div>
      {showRules ? (
        <div className="space-y-3">
          <div className="grid gap-2 sm:grid-cols-[minmax(0,1.1fr)_minmax(0,1fr)]">
            <Field>
              <FieldLabel htmlFor={selectId}>匹配表达式</FieldLabel>
              <Select
                value={draft.magic_id || (draft.pattern ? "__custom__" : "__all__")}
                items={[
                  ...magicRegex.map((rule) => ({
                    value: rule.id,
                    label: rule.name,
                  })),
                  ...(draft.magic_id || !draft.pattern
                    ? []
                    : [{ value: "__custom__", label: "自定义正则" }]),
                  { value: "__all__", label: "自定义 / 留空转存全部" },
                ]}
                onValueChange={(value) => {
                  if (value === "__custom__") return;
                  setDraft((prev) => ({
                    ...prev,
                    magic_id: value && value !== "__all__" ? value : "",
                    pattern: "",
                  }));
                }}
              >
                <SelectTrigger id={selectId}>
                  <SelectValue placeholder="自定义 / 留空转存全部" />
                </SelectTrigger>
                <SelectContent>
                  {magicRegex.map((rule) => (
                    <SelectItem key={rule.id} value={rule.id} className="py-1.5">
                      <div className="min-w-0">
                        <div className="truncate font-mono text-[13px] text-ink">
                          {rule.name}
                        </div>
                        <div className="mt-0.5 truncate font-mono text-[12px] text-muted-ink/70">
                          {rule.pattern || "未填写正则"}
                        </div>
                      </div>
                    </SelectItem>
                  ))}
                  {draft.magic_id || !draft.pattern ? null : (
                    <SelectItem value="__custom__">
                      <div className="min-w-0">
                        <div className="text-[13px] text-ink">自定义正则</div>
                        <div className="mt-0.5 truncate font-mono text-[12px] text-muted-ink/70">
                          {draft.pattern}
                        </div>
                      </div>
                    </SelectItem>
                  )}
                  <SelectItem value="__all__">自定义 / 留空转存全部</SelectItem>
                </SelectContent>
              </Select>
            </Field>
            <Field>
              <FieldLabel htmlFor={replaceId}>替换表达式</FieldLabel>
              <Input
                id={replaceId}
                value={draft.replace}
                onChange={(event) => update("replace", event.target.value)}
                placeholder="留空使用魔法变量"
              />
            </Field>
          </div>
          <div className="rounded-md border border-line bg-line-soft/40 px-2.5 py-2">
            <div className="font-mono text-[12px] uppercase tracking-[0.08em] text-subtle">
              PREVIEW
            </div>
            <div className="mt-1 flex items-center gap-2 overflow-hidden font-mono text-[12px]">
              <span className="truncate text-muted-ink">Black.Mirror.S07E05.mkv</span>
              <span className="text-subtle">→</span>
              <span className="truncate font-medium text-ink">
                {samplePreview || "计算中…"}
              </span>
            </div>
          </div>
          <SwitchField
            label="忽略文件后缀"
            description="01.mp4 与 01.mkv 视为同一文件，避免重复转存。"
            checked={Boolean(draft.ignore_extension)}
            onCheckedChange={(checked) => update("ignore_extension", checked)}
          />
        </div>
      ) : null}
    </section>
  );
}
