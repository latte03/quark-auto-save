import type { AppConfig, Task } from "../types";
import { post, request } from "./client";

function normalizeConfig(raw: AppConfig): AppConfig {
  const source = (raw.source ?? {}) as Partial<AppConfig["source"]>;
  const localDownload = (raw.local_download ??
    {}) as Partial<AppConfig["local_download"]>;
  return {
    ...raw,
    cookie: raw.cookie ?? [],
    next_runs: raw.next_runs ?? [],
    push_config: raw.push_config ?? {},
    plugins: raw.plugins ?? {},
    magic_regex: raw.magic_regex ?? [],
    plugin_status: raw.plugin_status ?? {},
    webui: raw.webui ?? { username: "admin" },
    source: {
      cloudsaver: {
        enable: false,
        server: "",
        username: "",
        password: "",
        token: "",
        ...(source.cloudsaver ?? {}),
      },
      pansou: { enable: false, server: "", ...(source.pansou ?? {}) },
    },
    local_download: {
      dir: "",
      concurrency: 2,
      min_free_gb: 5,
      ...localDownload,
    },
    run_logs: {
      keep: 20,
      ...((raw.run_logs ?? {}) as Partial<AppConfig["run_logs"]>),
    },
  };
}

export async function getConfig() {
  return normalizeConfig(await request<AppConfig>("/api/config"));
}

export async function saveConfig(partial: Partial<AppConfig>) {
  return normalizeConfig(
    await request<AppConfig>("/api/config", {
      method: "PUT",
      body: JSON.stringify(partial),
    }),
  );
}

export function renameMagicRule(id: string, name: string) {
  return post<{ tasks: Task[] }>("/api/magic/rename", { id, name });
}
