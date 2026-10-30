import { useEffect, useState } from "react";
import { ChevronDown } from "lucide-react";
import { pluginLabels } from "../../lib/mock";
import { cn } from "../../lib/utils";
import { Badge } from "../../components/ui/badge";
import {
  CodeBlock,
  CodeBlockActions,
  CodeBlockContent,
  CodeBlockCopyButton,
  CodeBlockHeader,
  CodeBlockTitle,
} from "../../components/ui/code-block";
import { sectionTitle, pluginEnabled } from "./constants";
import { PluginFields } from "./fields";
import { useTaskDraftContext } from "./TaskDraftContext";

export function PluginsSection({ open }: { open: boolean }) {
  const { draft, updateAddition, pluginDefaults } = useTaskDraftContext();
  const [showPlugins, setShowPlugins] = useState(false);
  const [showRaw, setShowRaw] = useState(false);
  const [openPlugin, setOpenPlugin] = useState<string | null>(
    Object.keys(draft.addition)[0] ?? null,
  );

  useEffect(() => {
    if (!open) return;
    setShowPlugins(false);
    setShowRaw(false);
    setOpenPlugin(Object.keys(draft.addition)[0] ?? null);
    // Reset the panels once per sheet opening; draft.addition then tracks edits.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open]);

  const pluginKeys = Array.from(
    new Set([...Object.keys(pluginDefaults), ...Object.keys(draft.addition)]),
  );
  const enabledPluginCount = pluginKeys.filter((plugin) =>
    pluginEnabled(plugin, {
      ...(pluginDefaults[plugin] ?? {}),
      ...(draft.addition[plugin] ?? {}),
    }),
  ).length;
  const rawJson = JSON.stringify(draft.addition, null, 2);

  return (
    <>
      <section className="space-y-2 border-t border-line pt-3.5">
        <button
          type="button"
          aria-expanded={showPlugins}
          onClick={() => setShowPlugins((prev) => !prev)}
          className="flex w-full items-center justify-between gap-2 text-left"
        >
          <h3 className={sectionTitle}>插件选项</h3>
          <span className="flex items-center gap-1.5 text-[11px] text-subtle">
            {enabledPluginCount}/{pluginKeys.length} 已启用
            <ChevronDown
              className={cn(
                "h-3.5 w-3.5 shrink-0 transition-transform",
                showPlugins && "rotate-180",
              )}
            />
          </span>
        </button>
        {showPlugins ? (
          <div className="space-y-2">
            {pluginKeys.map((plugin) => {
              const values = {
                ...(pluginDefaults[plugin] ?? {}),
                ...(draft.addition[plugin] ?? {}),
              };
              if (Object.keys(values).length === 0) return null;
              const enabled = pluginEnabled(plugin, values);
              const isOpen = openPlugin === plugin;
              return (
                <div
                  key={plugin}
                  className="overflow-hidden rounded-md border border-line bg-panel"
                >
                  <button
                    type="button"
                    aria-expanded={isOpen}
                    onClick={() => setOpenPlugin(isOpen ? null : plugin)}
                    className="flex w-full items-center gap-2 px-3 py-2 text-left transition-colors hover:bg-line-soft/40"
                  >
                    <span className="min-w-0 flex-1 truncate text-[12px] font-medium text-ink">
                      {pluginLabels[plugin] ?? plugin}
                    </span>
                    <span className="hidden font-mono text-[12px] uppercase tracking-[0.08em] text-subtle sm:inline">
                      {plugin}
                    </span>
                    <Badge variant={enabled ? "ok" : "neutral"} dot>
                      {enabled ? "已启用" : "未启用"}
                    </Badge>
                    <ChevronDown
                      className={cn(
                        "h-3.5 w-3.5 shrink-0 text-subtle transition-transform",
                        isOpen && "rotate-180",
                      )}
                    />
                  </button>
                  {isOpen ? (
                    <div className="border-t border-line-soft px-3 py-2">
                      <PluginFields
                        values={values}
                        onChange={(key, value) =>
                          updateAddition(plugin, key, value)
                        }
                      />
                    </div>
                  ) : null}
                </div>
              );
            })}
          </div>
        ) : null}
      </section>

      <section className="border-t border-line pt-3.5">
        <button
          type="button"
          aria-expanded={showRaw}
          onClick={() => setShowRaw((prev) => !prev)}
          className="flex w-full items-center justify-between text-[12px] text-muted-ink transition-colors hover:text-ink"
        >
          原始配置 JSON
          <ChevronDown
            className={cn(
              "h-3.5 w-3.5 text-subtle transition-transform",
              showRaw && "rotate-180",
            )}
          />
        </button>
        {showRaw ? (
          <CodeBlock className="mt-2">
            <CodeBlockHeader>
              <CodeBlockTitle>插件覆盖项</CodeBlockTitle>
              <CodeBlockActions>
                <CodeBlockCopyButton value={rawJson} />
              </CodeBlockActions>
            </CodeBlockHeader>
            <CodeBlockContent className="max-h-64" aria-label="任务插件配置 JSON">
              {rawJson}
            </CodeBlockContent>
          </CodeBlock>
        ) : null}
      </section>
    </>
  );
}
