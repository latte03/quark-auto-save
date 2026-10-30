import { useEffect, useRef, useState } from "react";
import { Field, FieldDescription, FieldLabel } from "../../components/ui/field";
import { Input } from "../../components/ui/input";
import { PageSection } from "../../components/PageSection";
import { useSettings } from "./SettingsContext";

const MIN_KEEP = 1;
const MAX_KEEP = 200;

function clampKeep(value: number) {
  return Math.min(MAX_KEEP, Math.max(MIN_KEEP, value));
}

export function LogsSection() {
  const { config, update } = useSettings();
  // 数字输入保留本地草稿：清空输入框不能被当成 0，但合法值要即时写回，
  // 否则未失焦就关页面时脏标记和离开提醒都看不到这次修改。
  const [keepText, setKeepText] = useState(String(config.run_logs.keep));
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (document.activeElement === inputRef.current) return;
    setKeepText(String(config.run_logs.keep));
  }, [config.run_logs.keep]);

  const commit = (raw: string) => {
    const parsed = Number.parseInt(raw, 10);
    if (!Number.isFinite(parsed)) return;
    const keep = clampKeep(parsed);
    if (keep !== config.run_logs.keep) update("run_logs", { keep });
  };

  return (
    <PageSection
      id="settings-logs"
      title="日志"
      description="任务与签到的输出会保存在 NAS 上，运行记录页按时间倒序展示。"
    >
      <Field className="max-w-[240px]">
        <FieldLabel htmlFor="run-log-keep">运行日志保留条数</FieldLabel>
        <Input
          ref={inputRef}
          id="run-log-keep"
          type="number"
          min={MIN_KEEP}
          max={MAX_KEEP}
          value={keepText}
          onChange={(event) => {
            setKeepText(event.target.value);
            commit(event.target.value);
          }}
          onBlur={() => {
            const parsed = Number.parseInt(keepText, 10);
            setKeepText(
              String(Number.isFinite(parsed) ? clampKeep(parsed) : config.run_logs.keep),
            );
          }}
          onKeyDown={(event) => {
            if (event.key === "Enter") event.currentTarget.blur();
          }}
        />
        <FieldDescription>超出后自动删除最旧记录。</FieldDescription>
      </Field>
    </PageSection>
  );
}
