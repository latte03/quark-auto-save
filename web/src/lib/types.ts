export type Page = "overview" | "tasks" | "files" | "runs" | "settings";

export type TaskStatus = "tracking" | "waiting" | "done" | "banned";

export type TaskProgress = {
  saved: number;
  expected: number;
  days: number[];
};

export type Task = {
  index?: number;
  taskname: string;
  shareurl: string;
  savepath: string;
  pattern: string;
  replace: string;
  update_subdir?: string;
  update_subdir_resave_mode?: boolean;
  ignore_extension?: boolean;
  startfid?: string;
  enddate?: string;
  runweek: number[];
  addition: Record<string, Record<string, string | number | boolean>>;
  shareurl_ban?: string;
  local_download?: TaskDownload;
  latest: string;
  next_run: string;
  status: TaskStatus;
  progress: TaskProgress;
};

export type TaskDownload = {
  enable: boolean;
  dir?: string;
};

export type DownloadJob = {
  id: string;
  task_name: string;
  root: string;
  status: "queued" | "running" | "done" | "failed";
  total: number;
  done: number;
  failed: number;
  skipped: number;
  bytes: number;
  message?: string;
  started_at?: string;
  finished_at?: string;
};

export type MagicRegex = {
  pattern: string;
  replace: string;
};

export type PluginConfig = {
  [key: string]: string | number | boolean;
};

export type SearchResult = {
  shareurl: string;
  taskname: string;
  content: string;
  datetime: string;
  channel: string;
  source: string;
  tags?: string[];
  verify?: boolean;
};

export type ShareFile = {
  name: string;
  fid: string;
  isDir: boolean;
  size?: number;
  category?: string;
  saved?: boolean;
  renamed?: string;
};

export type DirEntry = {
  name: string;
  fid: string;
  isDir: boolean;
  count?: number;
};

export type SourceConfig = {
  cloudsaver: {
    enable: boolean;
    server: string;
    username: string;
    password: string;
    token: string;
  };
  pansou: {
    enable: boolean;
    server: string;
  };
};

export type LocalDownload = {
  dir: string;
  concurrency: number;
  min_free_gb: number;
  overwrite: boolean;
};

export type RunLogSettings = {
  keep: number;
};

export type AppConfig = {
  cookie: string[];
  crontab: string;
  next_runs: string[];
  version: string;
  api_token: string;
  plugin_status: Record<string, boolean>;
  webui: { username: string };
  push_config: Record<string, string | number | boolean>;
  plugins: Record<string, PluginConfig>;
  magic_regex: Record<string, MagicRegex>;
  source: SourceConfig;
  local_download: LocalDownload;
  run_logs: RunLogSettings;
};

export type TasksPayload = {
  tasks: Task[];
  task_plugins_default: Record<string, PluginConfig>;
  plugin_status: Record<string, boolean>;
  version: string;
  crontab: string;
  running: string;
};

export type RunView = {
  id: string;
  label: string;
  started_at: string;
  finished_at?: string;
  done: boolean;
  dry?: boolean;
  error?: string;
  saved_total: number;
  tasks?: Record<string, number>;
};

export type TaskPluginDefaults = Record<string, PluginConfig>;
