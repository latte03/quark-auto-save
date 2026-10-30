import { useEffect, useState } from "react";
import { Field, FieldDescription, FieldLabel } from "../../components/ui/field";
import { Input } from "../../components/ui/input";
import { PageSection } from "../../components/PageSection";
import { useSettings } from "./SettingsContext";

const MIN_KEEP = 1;
const MAX_KEEP = 200;

export function LogsSection() {
  const { config, update } = useSettings();
  // 数字输入保留本地草稿，只在失焦/回车时写回，避免清空输入框时被当成 0。
  const [keepText, setKeepText] = useState(String(config.run_logs.keep));

  useEffect(() => {
    setKeepText(String(config.run_logs.keep));
  }, [config.run_logs.keep]);

  const commitKeep = () => {
    const parsed = Number.parseInt(keepText, 10);
    if (!Number.isFinite(parsed)) {
      setKeepText(String(config.run_logs.keep));
      return;
    }
    const keep = Math.min(MAX_KEEP, Math.max(MIN_KEEP, parsed));
    setKeepText(String(keep));
    update("run_logs", { keep });
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
          id="run-log-keep"
          type="number"
          min={MIN_KEEP}
          max={MAX_KEEP}
          value={keepText}
          onChange={(event) => setKeepText(event.target.value)}
          onBlur={commitKeep}
          onKeyDown={(event) => {
            if (event.key === "Enter") event.currentTarget.blur();
          }}
        />
        <FieldDescription>超出后自动删除最旧记录。</FieldDescription>
      </Field>
    </PageSection>
  );
}
