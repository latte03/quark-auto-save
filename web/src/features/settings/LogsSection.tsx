import { Field, FieldDescription, FieldLabel } from "../../components/ui/field";
import { Input } from "../../components/ui/input";
import { PageSection } from "../../components/PageSection";
import { useSettings } from "./SettingsContext";

export function LogsSection() {
  const { config, update } = useSettings();

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
          min={1}
          max={200}
          value={config.run_logs.keep}
          onChange={(event) =>
            update("run_logs", { keep: Number(event.target.value) })
          }
        />
        <FieldDescription>超出后自动删除最旧记录。</FieldDescription>
      </Field>
    </PageSection>
  );
}
