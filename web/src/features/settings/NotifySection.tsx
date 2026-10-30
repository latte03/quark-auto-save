import { useState } from "react";
import { Bell, Loader2, Plus, Trash2 } from "lucide-react";
import { testNotify } from "../../lib/api";
import { Button } from "../../components/ui/button";
import { Input } from "../../components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "../../components/ui/select";
import { PageSection } from "../../components/PageSection";
import { useToast } from "../../components/ToastProvider";
import { isSecret, pushLabels } from "./constants";
import { useAsyncAction } from "./useAsyncAction";
import { useSettings } from "./SettingsContext";

export function NotifySection() {
  const { config, setConfig } = useSettings();
  const notify = useToast();
  const [newChannel, setNewChannel] = useState("");
  const [testing, runTest] = useAsyncAction();
  const [notifyLog, setNotifyLog] = useState("");

  const handleTest = () =>
    runTest(async () => {
      try {
        const result = await testNotify();
        const failed = result.channels.filter((item) => !item.ok);
        const ok = result.channels.length - failed.length;
        notify(
          `推送测试：${ok}/${result.channels.length} 成功${
            failed.length > 0
              ? `，${failed.map((item) => item.channel).join("、")} 失败`
              : ""
          }`,
          failed.length > 0 ? "warn" : "ok",
        );
        setNotifyLog(result.log.trim());
      } catch (cause) {
        notify(cause instanceof Error ? cause.message : "推送测试失败", "error");
      }
    });

  return (
    <PageSection
      id="settings-notify"
      title="通知"
      description="任务结束、签到结果与失效提醒统一汇总；支持多渠道并行推送。"
      action={
        <Button
          variant="secondary"
          size="sm"
          disabled={testing || Object.keys(config.push_config).length === 0}
          onClick={() => void handleTest()}
        >
          {testing ? (
            <Loader2 className="h-3.5 w-3.5 animate-spin" />
          ) : (
            <Bell className="h-3.5 w-3.5" />
          )}
          发送测试
        </Button>
      }
    >
      {Object.keys(config.push_config).length > 0 ? (
        <div className="space-y-1 rounded-md bg-line-soft/50 p-2">
          {Object.entries(config.push_config).map(([key, value]) => (
            <div key={key} className="flex items-center gap-3 px-1.5 py-1.5">
              <span className="w-[132px] shrink-0 text-[12px] font-medium text-ink">
                {pushLabels[key] ?? key}
              </span>
              <span className="font-mono text-[12px] uppercase tracking-[0.08em] text-subtle">
                {key}
              </span>
              <Input
                value={String(value)}
                onChange={(event) =>
                  setConfig((prev) => ({
                    ...prev,
                    push_config: { ...prev.push_config, [key]: event.target.value },
                  }))
                }
                type={isSecret(key) ? "password" : "text"}
                className="ml-auto h-7 max-w-[320px] font-mono text-[12px]"
                aria-label={key}
              />
              <Button
                variant="ghost"
                size="icon-sm"
                aria-label={`删除 ${key}`}
                onClick={() =>
                  setConfig((prev) => {
                    const next = { ...prev.push_config };
                    delete next[key];
                    return { ...prev, push_config: next };
                  })
                }
              >
                <Trash2 className="h-3.5 w-3.5" />
              </Button>
            </div>
          ))}
        </div>
      ) : (
        <p className="text-[12px] text-muted-ink">
          还没有添加通知渠道；从下面选择一项后填写参数。
        </p>
      )}
      <div className="mt-2 flex gap-2">
        <Select
          value={newChannel || "__none__"}
          items={[
            { value: "__none__", label: "添加通知渠道…" },
            ...Object.keys(pushLabels)
              .filter((key) => !(key in config.push_config))
              .map((key) => ({ value: key, label: pushLabels[key] ?? key })),
          ]}
          onValueChange={(value) =>
            setNewChannel(!value || value === "__none__" ? "" : value)
          }
        >
          <SelectTrigger className="max-w-[240px]" aria-label="选择要添加的通知渠道">
            <SelectValue placeholder="添加通知渠道…" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="__none__">添加通知渠道…</SelectItem>
            {Object.keys(pushLabels)
              .filter((key) => !(key in config.push_config))
              .map((key) => (
                <SelectItem key={key} value={key}>
                  {pushLabels[key]}
                </SelectItem>
              ))}
          </SelectContent>
        </Select>
        <Button
          variant="secondary"
          size="sm"
          disabled={!newChannel}
          onClick={() => {
            if (!newChannel) return;
            setConfig((prev) => ({
              ...prev,
              push_config: { ...prev.push_config, [newChannel]: "" },
            }));
            setNewChannel("");
            notify("已添加通知渠道，填写后保存", "ok");
          }}
        >
          <Plus className="h-3.5 w-3.5" />
          添加
        </Button>
      </div>
      {notifyLog ? (
        <pre className="mt-2 max-h-48 overflow-auto whitespace-pre-wrap rounded-md bg-line-soft/50 p-2.5 font-mono text-[12px] leading-relaxed text-muted-ink">
          {notifyLog}
        </pre>
      ) : null}
    </PageSection>
  );
}
