import { useState } from "react";
import { ChevronDown, Loader2, Plug, Radar } from "lucide-react";
import { pluginDescriptions, pluginLabels } from "../../lib/mock";
import type { PluginConfig } from "../../lib/types";
import { cn } from "../../lib/utils";
import { Badge } from "../../components/ui/badge";
import { Button } from "../../components/ui/button";
import { SwitchField } from "../../components/SwitchField";
import { PluginNumberField, PluginTextField } from "../../components/PluginField";
import { PageSection } from "../../components/PageSection";
import { isSecret, pluginFieldLabels } from "./constants";
import { useSettings } from "./SettingsContext";

function fieldLabel(key: string) {
  return pluginFieldLabels[key] ?? key;
}

function PluginFields({
  plugin,
  values,
  onChange,
}: {
  plugin: string;
  values: PluginConfig;
  onChange: (key: string, value: string | number | boolean) => void;
}) {
  return (
    <div className="grid gap-x-3 md:grid-cols-2">
      {Object.entries(values).map(([key, value]) => {
        const label = fieldLabel(key);
        if (typeof value === "boolean") {
          return (
            <div key={key} className="md:col-span-1">
              <SwitchField
                label={label}
                checked={value}
                onCheckedChange={(checked) => onChange(key, checked)}
              />
            </div>
          );
        }
        if (typeof value === "number") {
          return (
            <PluginNumberField
              key={key}
              label={label}
              value={value}
              onChange={(next) => onChange(key, next)}
            />
          );
        }
        return (
          <PluginTextField
            key={key}
            label={label}
            secret={isSecret(key)}
            placeholder={isSecret(key) ? "已配置，修改后保存" : `${plugin} ${key}`}
            value={value}
            onChange={(next) => onChange(key, next)}
          />
        );
      })}
    </div>
  );
}

export function PluginsSection() {
  const { config, updatePlugin, pluginStatus, detection } = useSettings();
  const [openPlugin, setOpenPlugin] = useState<string | null>(
    Object.keys(config.plugins)[0] ?? null,
  );

  const pluginNames = Object.keys(config.plugins);
  const activePlugins = pluginNames.filter((plugin) => pluginStatus[plugin]).length;

  return (
    <PageSection
      id="settings-plugins"
      title="插件"
      description="按固定顺序在转存后运行；PLUGIN_FLAGS 环境变量可在启动时排除插件。"
      action={
        <div className="flex items-center gap-2">
          <Badge variant="ok" dot>
            已连接 {activePlugins} / {pluginNames.length}
          </Badge>
          <Button
            variant="ghost"
            size="sm"
            disabled={detection.alistBusy}
            onClick={() => void detection.runAlist()}
          >
            {detection.alistBusy ? (
              <Loader2 className="h-3.5 w-3.5 animate-spin" />
            ) : (
              <Radar className="h-3.5 w-3.5" />
            )}
            自动识别 Alist
          </Button>
        </div>
      }
      bodyClassName="p-0"
    >
      {detection.alistNote ? (
        <div className="bg-line-soft/50 px-4 py-2 text-[12px] text-muted-ink">
          {detection.alistNote}
        </div>
      ) : null}
      <div className="divide-y divide-line-soft">
        {pluginNames.map((plugin) => {
          const open = openPlugin === plugin;
          const values = config.plugins[plugin] ?? {};
          return (
            <div key={plugin}>
              <button
                type="button"
                className="flex w-full items-center gap-3 px-4 py-3 text-left transition-colors hover:bg-line-soft/30"
                onClick={() => setOpenPlugin(open ? null : plugin)}
                aria-expanded={open}
              >
                <span
                  className={cn(
                    "grid h-7 w-7 shrink-0 place-items-center rounded-md",
                    pluginStatus[plugin]
                      ? "bg-ok-soft text-ok"
                      : "bg-line-soft text-muted-ink",
                  )}
                >
                  <Plug className="h-3.5 w-3.5" />
                </span>
                <span className="min-w-0 flex-1">
                  <span className="flex items-center gap-1.5">
                    <span className="truncate text-[13px] font-medium text-ink">
                      {pluginLabels[plugin] ?? plugin}
                    </span>
                  </span>
                  <span className="mt-0.5 block truncate text-[12px] text-muted-ink">
                    {pluginDescriptions[plugin] ?? ""}
                  </span>
                </span>
                <Badge variant={pluginStatus[plugin] ? "ok" : "neutral"} dot>
                  {pluginStatus[plugin] ? "已连接" : "未配置"}
                </Badge>
                <ChevronDown
                  className={cn(
                    "h-3.5 w-3.5 shrink-0 text-subtle transition-transform",
                    open && "rotate-180",
                  )}
                />
              </button>
              {open ? (
                <div className="bg-line-soft/30 px-4 py-3">
                  <PluginFields
                    plugin={plugin}
                    values={values}
                    onChange={(key, value) => updatePlugin(plugin, key, value)}
                  />
                  <div className="mt-2 flex items-center justify-between">
                    <span className="font-mono text-[12px] uppercase tracking-[0.08em] text-subtle">
                      {plugin} · default_config
                    </span>
                  </div>
                </div>
              ) : null}
            </div>
          );
        })}
      </div>
    </PageSection>
  );
}
