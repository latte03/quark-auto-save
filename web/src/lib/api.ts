import type {
  AppConfig,
  RunView,
  SearchResult,
  Task,
  TasksPayload,
} from "./types";

export class ApiError extends Error {
  status: number;

  constructor(message: string, status = 0) {
    super(message);
    this.name = "ApiError";
    this.status = status;
  }
}

let unauthorizedHandler: (() => void) | null = null;

/** Called once per 401 so the shell can re-check the session and fall back to login. */
export function setUnauthorizedHandler(handler: (() => void) | null) {
  unauthorizedHandler = handler;
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  let response: Response;
  try {
    response = await fetch(path, {
      credentials: "include",
      ...init,
      headers: {
        ...(init?.body ? { "Content-Type": "application/json" } : {}),
        ...init?.headers,
      },
    });
  } catch {
    throw new ApiError("无法连接服务端，确认后端已启动");
  }
  let payload: { success?: boolean; message?: string; data?: T } | null = null;
  try {
    payload = await response.json();
  } catch {
    payload = null;
  }
  // A wrong password also answers 401, but it must stay on the login form.
  if (response.status === 401 && path !== "/api/login") {
    unauthorizedHandler?.();
  }
  if (!response.ok || payload?.success === false) {
    throw new ApiError(
      payload?.message ?? `请求失败 (${response.status})`,
      response.status,
    );
  }
  if (payload && typeof payload === "object" && "data" in payload) {
    return payload.data as T;
  }
  return payload as T;
}

function post<T>(path: string, body: unknown): Promise<T> {
  return request<T>(path, { method: "POST", body: JSON.stringify(body) });
}

// ---------- session ----------

export type SessionInfo = {
  authenticated: boolean;
  username: string;
  version: string;
};

export function getSession() {
  return request<SessionInfo>("/api/session");
}

export type LoginResult = { token: string; username: string };

export function login(username: string, password: string) {
  return post<LoginResult>("/api/login", { username, password });
}

export function logout() {
  return post<{ success: boolean }>("/api/logout", {});
}

// ---------- config ----------

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

export type LocalDirCheck = {
  ok: boolean;
  writable: boolean;
  free_bytes: number;
  message: string;
};

export function validateLocalDir(path: string) {
  return post<LocalDirCheck>("/api/local/validate", { path });
}

export function getAuthorizedDirs() {
  return request<{ paths: string[] }>("/api/local/authorized");
}

export type LocalDirBrowse = {
  roots: string[];
  path: string;
  parent: string;
  dirs: { name: string; path: string }[];
};

export function browseLocalDirs(path?: string) {
  const query = path ? `?path=${encodeURIComponent(path)}` : "";
  return request<LocalDirBrowse>(`/api/local/browse${query}`);
}

export function getRuns() {
  return request<{ runs: import("./types").RunView[] }>("/api/runs");
}

export function getRunLog(id: string) {
  return request<{
    run: import("./types").RunView;
    lines: string[];
    done: boolean;
  }>(`/api/runs/${encodeURIComponent(id)}/log`);
}

export function getDownloads() {
  return request<{ jobs: import("./types").DownloadJob[] }>("/api/downloads");
}

export function pauseDownload(id: string) {
  return post<{ success: boolean }>(
    `/api/downloads/${encodeURIComponent(id)}/pause`,
    {},
  );
}

export function resumeDownload(id: string) {
  return post<{ success: boolean }>(
    `/api/downloads/${encodeURIComponent(id)}/resume`,
    {},
  );
}

export function cancelDownload(id: string) {
  return post<{ success: boolean }>(
    `/api/downloads/${encodeURIComponent(id)}/cancel`,
    {},
  );
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
  return post<{ tasks: import("./types").Task[] }>("/api/magic/rename", {
    id,
    name,
  });
}

// ---------- quark login ----------

export type QuarkQR = { id: string; qr_url: string; expires_in: number };

export function startQuarkQR() {
  return post<QuarkQR>("/api/quark/qr", {});
}

export type QuarkQRStatus = {
  status: "waiting" | "confirmed" | "expired" | "error";
  nickname?: string;
  cookies?: string[];
  message?: string;
};

export function pollQuarkQR(id: string) {
  return request<QuarkQRStatus>(`/api/quark/qr/${encodeURIComponent(id)}`);
}

export type QuarkAccount = {
  ok: boolean;
  nickname?: string;
  checked_at?: string;
  message?: string;
};

export function getQuarkAccount() {
  return request<QuarkAccount>("/api/quark/account");
}

// ---------- tasks ----------

export function getTasks() {
  return request<TasksPayload>("/api/tasks");
}

export function createTask(task: Task) {
  return post<{ tasks: Task[] }>("/api/tasks", { task });
}

export function updateTask(index: number, task: Task) {
  return request<{ tasks: Task[] }>(`/api/tasks/${index}`, {
    method: "PUT",
    body: JSON.stringify({ task }),
  });
}

export function deleteTask(index: number) {
  return request<{ tasks: Task[] }>(`/api/tasks/${index}`, {
    method: "DELETE",
  });
}

// ---------- runs ----------

export function startRun(indices: number[], write: boolean, task?: Task) {
  return post<{ run_id: string }>("/api/run", {
    tasks: indices,
    write,
    ...(task ? { task } : {}),
  });
}

export function getLatestRun() {
  return request<RunView | null>("/api/runs/latest");
}

export function runStreamUrl(id: string) {
  return `/api/runs/${encodeURIComponent(id)}/stream`;
}

// ---------- browse / share / search ----------

export type BrowseEntry = {
  name: string;
  fid: string;
  is_dir: boolean;
  size: number;
  category?: string;
  count?: number;
};

export function browse(path: string) {
  return request<{ path: string; fid: string; entries: BrowseEntry[] }>(
    `/api/browse?path=${encodeURIComponent(path)}`,
  );
}

export type ShareValidation = {
  ok: boolean;
  title?: string;
  count?: number;
  message?: string;
};

export function validateShare(shareurl: string) {
  return post<ShareValidation>("/api/share/validate", { shareurl });
}

export type ShareDetailFile = {
  name: string;
  fid: string;
  is_dir: boolean;
  size: number;
  category?: string;
  matched: boolean;
  saved: boolean;
  renamed?: string;
};

export type ShareDetail = {
  title: string;
  savepath: string;
  pattern: string;
  replace: string;
  total: number;
  files: ShareDetailFile[];
};

export type ShareDetailRequest = {
  shareurl: string;
  pdir_fid?: string;
  taskname: string;
  pattern: string;
  replace: string;
  savepath: string;
  update_subdir?: string;
  ignore_extension?: boolean;
};

export function shareDetail(payload: ShareDetailRequest) {
  return post<ShareDetail>("/api/share/detail", payload);
}

export type RenamePreviewItem = {
  name: string;
  matched: boolean;
  renamed: string;
};

export function previewRename(payload: {
  taskname: string;
  magic_id?: string;
  pattern: string;
  replace: string;
  files: string[];
}) {
  return post<{
    pattern: string;
    replace: string;
    files: RenamePreviewItem[];
  }>("/api/rename/preview", payload);
}

export function searchShares(keyword: string, deep = false) {
  return request<{ keyword: string; results: SearchResult[]; messages: string[] }>(
    `/api/search?keyword=${encodeURIComponent(keyword)}${deep ? "&deep=1" : ""}`,
  );
}

export type SourceDetection = {
  cloudsaver: { online: boolean; server?: string };
  pansou: { online: boolean; server?: string };
  alist: { online: boolean; server?: string };
};

export function detectSources() {
  return request<SourceDetection>("/api/sources/detect");
}

// ---------- sign & notify ----------

export function startSignin() {
  return post<{ run_id: string }>("/api/sign", {});
}

export type NotifyTestChannel = {
  channel: string;
  ok: boolean;
  error?: string;
};

export function testNotify() {
  return post<{ channels: NotifyTestChannel[]; log: string }>(
    "/api/notify/test",
    {},
  );
}

// ---------- file management ----------

export function renameFile(path: string, name: string) {
  return post<{ fid: string; name: string }>("/api/files/rename", {
    path,
    name,
  });
}

export function deleteFile(path: string) {
  return post<{ fid: string; task_id: string }>("/api/files/delete", {
    path,
    confirm: true,
  });
}

export function createFolder(path: string) {
  return post<{ path: string; fid: string }>("/api/files/mkdir", { path });
}
