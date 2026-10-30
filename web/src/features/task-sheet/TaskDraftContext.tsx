import { createContext, useContext } from "react";
import type { AppConfig, PluginConfig, Task } from "../../lib/types";
import type { ShareSearch } from "./useShareSearch";
import type { LinkValidation } from "./useLinkValidation";

export type PickerMode = "savepath" | "share";

export type TaskDraftContextValue = {
  draft: Task;
  setDraft: React.Dispatch<React.SetStateAction<Task>>;
  update: <K extends keyof Task>(key: K, value: Task[K]) => void;
  updateAddition: (
    plugin: string,
    key: string,
    value: string | number | boolean,
  ) => void;
  magicRegex: AppConfig["magic_regex"];
  pluginDefaults: Record<string, PluginConfig>;
  search: ShareSearch;
  link: LinkValidation;
  searchReady: boolean;
  onOpenSettings: () => void;
  openPicker: (mode: PickerMode) => void;
};

const TaskDraftContext = createContext<TaskDraftContextValue | null>(null);

export const TaskDraftProvider = TaskDraftContext.Provider;

export function useTaskDraftContext() {
  const value = useContext(TaskDraftContext);
  if (!value) {
    throw new Error("useTaskDraftContext must be used inside TaskDraftProvider");
  }
  return value;
}
