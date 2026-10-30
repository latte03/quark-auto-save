import { useCallback, useEffect, useState } from "react";
import { createTask, deleteTask, getTasks, updateTask } from "../lib/api";
import type { PluginConfig, Task, TasksPayload } from "../lib/types";

export function useTaskCollection(polling: boolean) {
  const [tasks, setTasks] = useState<Task[]>([]);
  const [pluginDefaults, setPluginDefaults] = useState<Record<string, PluginConfig>>({});
  const [pluginStatus, setPluginStatus] = useState<Record<string, boolean>>({});

  const apply = useCallback((payload: TasksPayload) => {
    setTasks(payload.tasks);
    setPluginDefaults(payload.task_plugins_default ?? {});
    setPluginStatus(payload.plugin_status ?? {});
  }, []);

  const refresh = useCallback(async () => {
    const payload = await getTasks();
    apply(payload);
    return payload;
  }, [apply]);

  const save = useCallback(async (previous: Task | null, next: Task) => {
    const index = previous?.index ?? null;
    const payload =
      index != null ? await updateTask(index, next) : await createTask(next);
    setTasks(payload.tasks);
    // 新建的任务由后端追加到末尾，直接取返回列表的索引比按名字回查可靠。
    const created = payload.tasks.length - 1;
    return { payload, index: index ?? (created >= 0 ? created : null) };
  }, []);

  const remove = useCallback(async (task: Task) => {
    if (task.index == null) return;
    const payload = await deleteTask(task.index);
    setTasks(payload.tasks);
  }, []);

  const clear = useCallback(() => {
    setTasks([]);
    setPluginDefaults({});
    setPluginStatus({});
  }, []);

  // One-shot tasks complete on the server when their download finishes, after
  // the run stream is closed; a slow poll keeps the list in sync.
  useEffect(() => {
    if (!polling) return;
    const timer = window.setInterval(() => {
      void refresh().catch(() => undefined);
    }, 10000);
    return () => window.clearInterval(timer);
  }, [polling, refresh]);

  return { tasks, pluginDefaults, pluginStatus, apply, refresh, save, remove, clear };
}
