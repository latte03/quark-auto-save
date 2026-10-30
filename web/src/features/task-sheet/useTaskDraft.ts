import { useCallback, useEffect, useState } from "react";
import type { AppConfig, PluginConfig, Task } from "../../lib/types";
import { deepEqual } from "../../lib/compare";
import { useBeforeUnload } from "../../hooks/use-before-unload";
import { emptyTask } from "./constants";

type TaskDraftParams = {
  open: boolean;
  task: Task | null;
  magicRegex: AppConfig["magic_regex"];
  pluginDefaults: Record<string, PluginConfig>;
};

export function useTaskDraft({
  open,
  task,
  magicRegex,
  pluginDefaults,
}: TaskDraftParams) {
  const [draft, setDraft] = useState<Task>(emptyTask);
  const [baseline, setBaseline] = useState("");

  useEffect(() => {
    if (!open) return;
    const next = task
      ? structuredClone(task)
      : {
          ...structuredClone(emptyTask),
          magic_id: magicRegex[0]?.id ?? "",
        };
    setDraft(next);
    setBaseline(JSON.stringify(next));
  }, [open, task, magicRegex]);

  const dirty = open && baseline !== "" && !deepEqual(draft, JSON.parse(baseline));
  useBeforeUnload(dirty);

  const update = useCallback(
    <K extends keyof Task,>(key: K, value: Task[K]) =>
      setDraft((prev) => ({ ...prev, [key]: value })),
    [],
  );

  // Overrides equal to the plugin default are dropped, so flipping a switch
  // back leaves no stale override behind.
  const updateAddition = useCallback(
    (plugin: string, key: string, value: string | number | boolean) =>
      setDraft((prev) => {
        const defaults = pluginDefaults[plugin] ?? {};
        const merged: Record<string, string | number | boolean> = {
          ...(prev.addition[plugin] ?? {}),
          [key]: value,
        };
        for (const field of Object.keys(merged)) {
          if (field in defaults && String(merged[field]) === String(defaults[field])) {
            delete merged[field];
          }
        }
        const addition = { ...prev.addition };
        if (Object.keys(merged).length === 0) {
          delete addition[plugin];
        } else {
          addition[plugin] = merged;
        }
        return { ...prev, addition };
      }),
    [pluginDefaults],
  );

  return { draft, setDraft, dirty, update, updateAddition };
}
