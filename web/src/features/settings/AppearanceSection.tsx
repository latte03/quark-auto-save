import { PageSection } from "../../components/PageSection";
import { Tabs, TabsList, TabsTrigger } from "../../components/ui/tabs";
import { themeLabels, type ThemeMode } from "../../lib/theme";
import { useSettings } from "./SettingsContext";

export function AppearanceSection() {
  const { theme, onThemeChange } = useSettings();

  return (
    <PageSection
      id="settings-appearance"
      title="外观"
      description="只影响当前浏览器，不写入配置文件。"
    >
      <Tabs
        value={theme}
        onValueChange={(value) => onThemeChange(value as ThemeMode)}
        className="max-w-[320px]"
      >
        <TabsList className="w-full">
          {(Object.keys(themeLabels) as ThemeMode[]).map((mode) => (
            <TabsTrigger key={mode} value={mode} className="text-[12px]">
              {themeLabels[mode]}
            </TabsTrigger>
          ))}
        </TabsList>
      </Tabs>
    </PageSection>
  );
}
