import { useState } from "react";
import type { AppConfig } from "../lib/types";
import { PageHeader } from "../components/PageHeader";
import { useToast } from "../components/ToastProvider";
import type { ThemeMode } from "../lib/theme";
import { SettingsProvider } from "./settings/SettingsContext";
import { SettingsNav } from "./settings/SettingsNav";
import { SaveBar } from "./settings/SaveBar";
import { AppearanceSection } from "./settings/AppearanceSection";
import { CookieSection } from "./settings/CookieSection";
import { LogsSection } from "./settings/LogsSection";
import { NotifySection } from "./settings/NotifySection";
import { DownloadSection } from "./settings/DownloadSection";
import { PluginsSection } from "./settings/PluginsSection";
import { MagicSection } from "./settings/MagicSection";
import { SourceSection } from "./settings/SourceSection";
import type { SettingsSection } from "./settings/constants";

type SettingsPageProps = {
  config: AppConfig;
  setConfig: (updater: React.SetStateAction<AppConfig>) => void;
  pluginStatus: Record<string, boolean>;
  running: boolean;
  dirty: boolean;
  theme: ThemeMode;
  onThemeChange: (mode: ThemeMode) => void;
  onAdoptDefaults: () => void;
  onSign: () => void;
  onSave: () => Promise<void>;
  onTasksReload: () => void;
};

export function SettingsPage({
  config,
  setConfig,
  pluginStatus,
  running,
  dirty,
  theme,
  onThemeChange,
  onAdoptDefaults,
  onSign,
  onSave,
  onTasksReload,
}: SettingsPageProps) {
  const notify = useToast();
  const [section, setSection] = useState<SettingsSection>("cookie");
  const [saving, setSaving] = useState(false);

  const go = (id: SettingsSection) => {
    setSection(id);
    document.getElementById(`settings-${id}`)?.scrollIntoView({
      behavior: "smooth",
      block: "start",
    });
  };

  const save = async () => {
    setSaving(true);
    try {
      await onSave();
      notify("配置已保存，调度器已重载", "ok");
    } catch (cause) {
      notify(cause instanceof Error ? cause.message : "保存失败", "error");
    } finally {
      setSaving(false);
    }
  };

  return (
    <SettingsProvider
      config={config}
      setConfig={setConfig}
      pluginStatus={pluginStatus}
      running={running}
      theme={theme}
      onThemeChange={onThemeChange}
      onAdoptDefaults={onAdoptDefaults}
      onSign={onSign}
      onTasksReload={onTasksReload}
    >
      <div className="space-y-4">
        <PageHeader
          title="设置"
          description="账号、通知、插件与命名规则。"
        />
        <div className="grid gap-4 lg:grid-cols-[172px_minmax(0,1fr)]">
          <SettingsNav section={section} onNavigate={go} />
          <div className="min-w-0 space-y-4">
            <AppearanceSection />
            <CookieSection />
            <LogsSection />
            <NotifySection />
            <DownloadSection />
            <PluginsSection />
            <MagicSection />
            <SourceSection />
          </div>
        </div>
        <SaveBar dirty={dirty} saving={saving} onSave={() => void save()} />
      </div>
    </SettingsProvider>
  );
}
