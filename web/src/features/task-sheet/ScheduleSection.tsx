import { useEffect, useState } from "react";
import { ChevronDown } from "lucide-react";
import { weekLabels } from "../../lib/mock";
import { cn } from "../../lib/utils";
import { DatePicker } from "../../components/ui/date-picker";
import { Field, FieldDescription, FieldLabel } from "../../components/ui/field";
import { Input } from "../../components/ui/input";
import { SwitchField } from "../../components/SwitchField";
import { Tabs, TabsList, TabsTrigger } from "../../components/ui/tabs";
import { sectionTitle } from "./constants";
import { RunTimesField } from "./fields";
import { useTaskDraftContext } from "./TaskDraftContext";

export function ScheduleSection({ open }: { open: boolean }) {
  const { draft, update } = useTaskDraftContext();
  const [showAdvanced, setShowAdvanced] = useState(false);
  const onceMode = draft.mode === "once";

  useEffect(() => {
    if (open) setShowAdvanced(false);
  }, [open]);

  return (
    <section className="space-y-3 border-t border-line pt-3.5">
      <h3 className={sectionTitle}>排程</h3>
      <Field>
        <FieldLabel>运行方式</FieldLabel>
        <Tabs
          value={onceMode ? "once" : "scheduled"}
          onValueChange={(value) =>
            update("mode", value === "once" ? "once" : "scheduled")
          }
        >
          <TabsList className="w-full">
            <TabsTrigger value="scheduled" className="text-[12px]">
              定时循环
            </TabsTrigger>
            <TabsTrigger value="once" className="text-[12px]">
              仅执行一次
            </TabsTrigger>
          </TabsList>
        </Tabs>
        <FieldDescription>
          {onceMode
            ? "不参与定时调度；保存并执行跑完一轮后标记完成，之后可在列表里手动重跑。"
            : "到点后按星期和截止日期过滤，多个时刻各跑一轮。"}
        </FieldDescription>
      </Field>
      {onceMode ? null : (
        <>
          <Field>
            <FieldLabel>运行时间</FieldLabel>
            <RunTimesField
              value={draft.runtime ?? []}
              onChange={(next) => update("runtime", next)}
            />
            <FieldDescription>多个时刻各跑一轮。</FieldDescription>
          </Field>
          <div className="grid gap-3 sm:grid-cols-[minmax(0,1fr)_minmax(0,1.1fr)]">
            <Field>
              <FieldLabel>截止日期</FieldLabel>
              <DatePicker
                aria-label="截止日期"
                value={draft.enddate ?? ""}
                onChange={(value) => update("enddate", value)}
              />
              <FieldDescription>留空表示永久运行</FieldDescription>
            </Field>
            <Field>
              <FieldLabel>运行星期</FieldLabel>
              <div className="grid grid-cols-7 gap-1">
                {weekLabels.map((label, index) => {
                  const day = index + 1;
                  const checked = draft.runweek.includes(day);
                  return (
                    <button
                      key={day}
                      type="button"
                      aria-pressed={checked}
                      aria-label={`周${label}`}
                      onClick={() =>
                        update(
                          "runweek",
                          checked
                            ? draft.runweek.filter((item) => item !== day)
                            : [...draft.runweek, day].sort(),
                        )
                      }
                      className={cn(
                        "flex h-7 items-center justify-center rounded border text-[12px] transition-colors",
                        checked
                          ? "border-brand bg-brand font-medium text-white"
                          : "border-line bg-panel text-muted-ink hover:border-brand/40 hover:bg-brand-soft hover:text-brand-ink",
                      )}
                    >
                      {label}
                    </button>
                  );
                })}
              </div>
            </Field>
          </div>
        </>
      )}
      <button
        type="button"
        onClick={() => setShowAdvanced((prev) => !prev)}
        className="flex w-full items-center justify-between rounded-md border border-line bg-panel px-2.5 py-2 text-[12px] text-muted-ink hover:bg-line-soft"
      >
        <span>高级：起始文件、子目录更新</span>
        <ChevronDown
          className={cn(
            "h-3.5 w-3.5 transition-transform",
            showAdvanced && "rotate-180",
          )}
        />
      </button>
      {showAdvanced ? (
        <div className="space-y-3 rounded-md border border-line bg-line-soft/30 p-3">
          <Field>
            <FieldLabel>文件起始 fid</FieldLabel>
            <Input
              value={draft.startfid ?? ""}
              onChange={(event) => update("startfid", event.target.value)}
              placeholder="可选"
            />
            <FieldDescription>只转存修改日期晚于该文件的文件</FieldDescription>
          </Field>
          <Field>
            <FieldLabel>更新子目录</FieldLabel>
            <Input
              value={draft.update_subdir ?? ""}
              onChange={(event) => update("update_subdir", event.target.value)}
              placeholder="4k|1080p"
            />
            <FieldDescription>匹配子目录名的正则，多项用 | 分隔</FieldDescription>
          </Field>
          <SwitchField
            label="重存模式"
            description="删除该子目录后重新转存，适合大资源包；关闭时递归更新。"
            checked={Boolean(draft.update_subdir_resave_mode)}
            onCheckedChange={(checked) =>
              update("update_subdir_resave_mode", checked)
            }
          />
        </div>
      ) : null}
    </section>
  );
}
