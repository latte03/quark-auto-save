import { Loader2, Radar, Search } from "lucide-react";
import { searchShares } from "../../lib/api";
import { Button } from "../../components/ui/button";
import { Field, FieldLabel } from "../../components/ui/field";
import { Input } from "../../components/ui/input";
import { SwitchField } from "../../components/SwitchField";
import { PageSection } from "../../components/PageSection";
import { useToast } from "../../components/ToastProvider";
import { useAsyncAction } from "./useAsyncAction";
import { useSettings } from "./SettingsContext";

export function SourceSection() {
  const { config, updateSource, detection } = useSettings();
  const notify = useToast();
  const [testing, runTest] = useAsyncAction();

  const testSearch = () =>
    runTest(async () => {
      try {
        const result = await searchShares("测试");
        const suffix = result.messages.length
          ? `（${result.messages.join("；")}）`
          : "";
        notify(`搜索源返回 ${result.results.length} 条候选${suffix}`, "ok");
      } catch (cause) {
        notify(cause instanceof Error ? cause.message : "搜索失败", "error");
      }
    });

  return (
    <PageSection
      id="settings-source"
      title="搜索源"
      description="任务名称搜索会并行查询已启用的源，并按发布时间排序去重。"
      action={
        <div className="flex items-center gap-2">
          <Button
            variant="ghost"
            size="sm"
            disabled={detection.searchBusy}
            onClick={() => void detection.runSearch()}
          >
            {detection.searchBusy ? (
              <Loader2 className="h-3.5 w-3.5 animate-spin" />
            ) : (
              <Radar className="h-3.5 w-3.5" />
            )}
            自动识别
          </Button>
          <Button
            variant="secondary"
            size="sm"
            disabled={testing}
            onClick={() => void testSearch()}
          >
            {testing ? (
              <Loader2 className="h-3.5 w-3.5 animate-spin" />
            ) : (
              <Search className="h-3.5 w-3.5" />
            )}
            测试搜索
          </Button>
        </div>
      }
    >
      <div className="space-y-4">
        {detection.searchNote ? (
          <div className="rounded-md bg-line-soft/50 px-3 py-2 text-[12px] text-muted-ink">
            {detection.searchNote}
          </div>
        ) : null}
        <div className="rounded-md bg-line-soft/50 px-3 py-2.5">
          <SwitchField
            label="CloudSaver"
            description="需要服务器地址与账号；token 失效时自动重新登录。"
            checked={config.source.cloudsaver.enable}
            onCheckedChange={(checked) =>
              updateSource("cloudsaver", "enable", checked)
            }
          />
          <div className="grid gap-2 md:grid-cols-2">
            <Field>
              <FieldLabel>服务器</FieldLabel>
              <Input
                value={config.source.cloudsaver.server}
                onChange={(event) =>
                  updateSource("cloudsaver", "server", event.target.value)
                }
                className="font-mono text-[12px]"
              />
            </Field>
            <Field>
              <FieldLabel>用户名</FieldLabel>
              <Input
                value={config.source.cloudsaver.username}
                onChange={(event) =>
                  updateSource("cloudsaver", "username", event.target.value)
                }
              />
            </Field>
            <Field>
              <FieldLabel>密码</FieldLabel>
              <Input
                type="password"
                value={config.source.cloudsaver.password}
                onChange={(event) =>
                  updateSource("cloudsaver", "password", event.target.value)
                }
              />
            </Field>
            <Field>
              <FieldLabel>Token</FieldLabel>
              <Input
                type="password"
                value={config.source.cloudsaver.token}
                onChange={(event) =>
                  updateSource("cloudsaver", "token", event.target.value)
                }
              />
            </Field>
          </div>
        </div>
        <div className="rounded-md bg-line-soft/50 px-3 py-2.5">
          <SwitchField
            label="PanSou"
            description="只需服务器地址；按关键词返回夸克网盘分享。"
            checked={config.source.pansou.enable}
            onCheckedChange={(checked) =>
              updateSource("pansou", "enable", checked)
            }
          />
          <Field>
            <FieldLabel>服务器</FieldLabel>
            <Input
              value={config.source.pansou.server}
              onChange={(event) =>
                updateSource("pansou", "server", event.target.value)
              }
              className="font-mono text-[12px]"
            />
          </Field>
        </div>
      </div>
    </PageSection>
  );
}
