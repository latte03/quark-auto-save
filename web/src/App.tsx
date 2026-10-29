import { useCallback, useEffect, useRef, useState, type SetStateAction } from "react";
import { Loader2 } from "lucide-react";
import { Sidebar } from "./components/layout/Sidebar";
import { Topbar } from "./components/layout/Topbar";
import { Toast, type ToastData } from "./components/Toast";
import {
  Dialog,
  DialogBody,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "./components/ui/dialog";
import { Button } from "./components/ui/button";
import { TooltipProvider } from "./components/ui/tooltip";
import { OverviewPage } from "./features/OverviewPage";
import { TasksPage, type TaskFilter } from "./features/TasksPage";
import { FilesPage } from "./features/FilesPage";
import { RunsPage } from "./features/RunsPage";
import { SettingsPage } from "./features/SettingsPage";
import { TaskSheet } from "./features/TaskSheet";
import { RunLogDialog } from "./features/RunLogDialog";
import { LoginPage } from "./features/LoginPage";
import {
  ApiError,
  createTask,
  deleteTask,
  getConfig,
  getLatestRun,
  getSession,
  getTasks,
  login,
  logout,
  runStreamUrl,
  saveConfig,
  startRun,
  startSignin,
  updateTask,
  type SessionInfo,
} from "./lib/api";
import type { AppConfig, Page, PluginConfig, RunView, Task, TasksPayload } from "./lib/types";

export default function App() {
  const [session, setSession] = useState<SessionInfo | null>(null);
  const [bootError, setBootError] = useState("");
  const [config, setConfig] = useState<AppConfig | null>(null);
  const [tasks, setTasks] = useState<Task[]>([]);
  const [pluginDefaults, setPluginDefaults] = useState<Record<string, PluginConfig>>({});
  const [pluginStatus, setPluginStatus] = useState<Record<string, boolean>>({});
  const [latestRun, setLatestRun] = useState<RunView | null>(null);

  const [page, setPage] = useState<Page>("overview");
  const [mobileNav, setMobileNav] = useState(false);
  const [taskFilter, setTaskFilter] = useState<TaskFilter>("all");
  const [query, setQuery] = useState("");
  const [taskSheet, setTaskSheet] = useState<{ open: boolean; task: Task | null }>({
    open: false,
    task: null,
  });
  const [deleteTarget, setDeleteTarget] = useState<Task | null>(null);
  const [logOpen, setLogOpen] = useState(false);
  const [dryRun, setDryRun] = useState(false);
  const [log, setLog] = useState<string[]>([]);
  const [running, setRunning] = useState(false);
  const [toast, setToast] = useState<ToastData | null>(null);

  const toastTimer = useRef<number | null>(null);
  const streamRef = useRef<EventSource | null>(null);
  const lastRunRef = useRef<{
    indices: number[];
    write: boolean;
    task?: Task;
    sign?: boolean;
  }>({ indices: [], write: true });
  const booted = useRef(false);

  const notify = useCallback((message: string, tone: ToastData["tone"] = "info") => {
    setToast({ id: Date.now(), message, tone });
    if (toastTimer.current) window.clearTimeout(toastTimer.current);
    toastTimer.current = window.setTimeout(() => setToast(null), 3200);
  }, []);

  const applyTasks = useCallback((payload: TasksPayload) => {
    setTasks(payload.tasks);
    setPluginDefaults(payload.task_plugins_default ?? {});
    setPluginStatus(payload.plugin_status ?? {});
  }, []);

  const refreshTasks = useCallback(async () => {
    const payload = await getTasks();
    applyTasks(payload);
  }, [applyTasks]);

  const closeStream = useCallback(() => {
    streamRef.current?.close();
    streamRef.current = null;
  }, []);

  const attachRun = useCallback(
    (id: string) => {
      closeStream();
      setLog([]);
      setRunning(true);
      setLogOpen(true);
      const source = new EventSource(runStreamUrl(id));
      streamRef.current = source;
      source.onmessage = (event) => {
        if (event.data === "[DONE]") {
          source.close();
          if (streamRef.current === source) streamRef.current = null;
          setRunning(false);
          void refreshTasks()
            .catch(() => undefined)
            .finally(() => {
              void getLatestRun()
                .then((run) => setLatestRun(run))
                .catch(() => undefined);
            });
          return;
        }
        setLog((prev) => [...prev, event.data]);
      };
      source.onerror = () => {
        // EventSource reconnects on its own with Last-Event-ID; only a closed
        // stream means it gave up (e.g. 401).
        if (source.readyState === EventSource.CLOSED) {
          if (streamRef.current === source) streamRef.current = null;
          setRunning(false);
        }
      };
    },
    [closeStream, refreshTasks],
  );

  const bootstrap = useCallback(async () => {
    try {
      const info = await getSession();
      setSession(info);
      if (!info.authenticated) return;
      const [cfg, payload] = await Promise.all([getConfig(), getTasks()]);
      setConfig(cfg);
      applyTasks(payload);
      void getLatestRun()
        .then((run) => setLatestRun(run))
        .catch(() => undefined);
      if (payload.running) attachRun(payload.running);
    } catch (cause) {
      setBootError(cause instanceof Error ? cause.message : "加载失败");
      setSession({ authenticated: false, username: "", version: "" });
    }
  }, [applyTasks, attachRun]);

  useEffect(() => {
    if (booted.current) return;
    booted.current = true;
    void bootstrap();
  }, [bootstrap]);

  useEffect(
    () => () => {
      if (toastTimer.current) window.clearTimeout(toastTimer.current);
      streamRef.current?.close();
    },
    [],
  );

  const handleLogin = async (username: string, password: string) => {
    try {
      await login(username, password);
    } catch (cause) {
      throw cause instanceof Error ? cause : new Error("登录失败");
    }
    booted.current = false;
    setBootError("");
    await bootstrap();
  };

  const handleLogout = async () => {
    closeStream();
    try {
      await logout();
    } catch {
      // The cookie is cleared server-side anyway on the next login.
    }
    setConfig(null);
    setTasks([]);
    setLatestRun(null);
    setSession({ authenticated: false, username: "", version: session?.version ?? "" });
  };

  const runTasks = useCallback(
    async (indices: number[], write: boolean) => {
      if (running) {
        notify("已有任务在运行，等这轮结束", "warn");
        return;
      }
      lastRunRef.current = { indices, write };
      try {
        setDryRun(!write);
        const { run_id } = await startRun(indices, write);
        attachRun(run_id);
      } catch (cause) {
        if (cause instanceof ApiError) {
          notify(cause.message, "warn");
          if (cause.status === 409) {
            void getTasks()
              .then((payload) => {
                applyTasks(payload);
                if (payload.running) attachRun(payload.running);
              })
              .catch(() => undefined);
          }
          return;
        }
        notify(cause instanceof Error ? cause.message : "启动失败", "warn");
      }
    },
    [applyTasks, attachRun, notify, running],
  );

  const runInline = useCallback(
    async (task: Task) => {
      if (running) {
        notify("已有任务在运行，等这轮结束", "warn");
        return;
      }
      try {
        lastRunRef.current = { indices: [], write: false, task };
        setDryRun(true);
        const { run_id } = await startRun([], false, task);
        attachRun(run_id);
      } catch (cause) {
        notify(cause instanceof Error ? cause.message : "试运行启动失败", "warn");
      }
    },
    [attachRun, notify, running],
  );

  const runSign = useCallback(async () => {    if (running) {
      notify("已有任务在运行，等这轮结束", "warn");
      return;
    }
    try {
      lastRunRef.current = { indices: [], write: true, sign: true };
      const { run_id } = await startSignin();
      attachRun(run_id);
    } catch (cause) {
      notify(cause instanceof Error ? cause.message : "签到启动失败", "warn");
    }
  }, [attachRun, notify, running]);

  const rerun = useCallback(() => {
    const last = lastRunRef.current;
    if (last.sign) {
      void runSign();
      return;
    }
    if (last.task) {
      void runInline(last.task);
      return;
    }
    void runTasks(last.indices, last.write);
  }, [runInline, runSign, runTasks]);

  const openLog = useCallback(async () => {
    setLogOpen(true);    if (running) return;
    try {
      const run = await getLatestRun();
      setLatestRun(run);
      setDryRun(Boolean(run?.dry));
      if (run) {
        attachRun(run.id);
      } else {
        setLog([]);
      }
    } catch (cause) {
      notify(cause instanceof Error ? cause.message : "读取运行记录失败", "warn");
    }
  }, [attachRun, notify, running]);

  const openTask = (task: Task) => setTaskSheet({ open: true, task });
  const openNewTask = () => setTaskSheet({ open: true, task: null });

  const handleSave = async (next: Task, runAfter: boolean) => {
    const previous = taskSheet.task;
    try {
      const payload =
        previous?.index != null
          ? await updateTask(previous.index, next)
          : await createTask(next);
      setTasks(payload.tasks);
      setTaskSheet({ open: false, task: null });
      notify(runAfter ? "任务已保存，开始运行" : "任务已保存", "ok");
      if (runAfter) {
        let index = previous?.index;
        if (index == null) {
          for (let cursor = payload.tasks.length - 1; cursor >= 0; cursor -= 1) {
            if (payload.tasks[cursor].taskname === next.taskname) {
              index = payload.tasks[cursor].index ?? cursor;
              break;
            }
          }
        }
        if (index != null) {
          await runTasks([index], true);
        }
      }
    } catch (cause) {
      notify(cause instanceof Error ? cause.message : "保存失败", "warn");
    }
  };

  const handleDelete = (task: Task) => setDeleteTarget(task);

  const confirmDelete = async () => {
    if (!deleteTarget || deleteTarget.index == null) return;
    try {
      const payload = await deleteTask(deleteTarget.index);
      setTasks(payload.tasks);
      setDeleteTarget(null);
      notify("任务已删除", "warn");
    } catch (cause) {
      notify(cause instanceof Error ? cause.message : "删除失败", "warn");
    }
  };

  const handleSettingsSave = async () => {
    if (!config) return;
    const saved = await saveConfig({
      cookie: config.cookie,
      crontab: config.crontab,
      push_config: config.push_config,
      plugins: config.plugins,
      magic_regex: config.magic_regex,
      source: config.source,
      local_download: config.local_download,
    });
    setConfig(saved);
    await refreshTasks();
  };

  const updateConfig = useCallback((updater: SetStateAction<AppConfig>) => {
    setConfig((prev) => {
      if (!prev) return prev;
      return typeof updater === "function" ? updater(prev) : updater;
    });
  }, []);

  const goTasks = (filter: TaskFilter = "all") => {
    setTaskFilter(filter);
    setPage("tasks");
  };

  if (!session) {
    return (
      <div className="grid min-h-screen place-items-center bg-canvas">
        <div className="flex items-center gap-2 text-[12px] text-muted">
          <Loader2 className="h-4 w-4 animate-spin" />
          {bootError || "正在连接控制台…"}
        </div>
      </div>
    );
  }

  if (!session.authenticated) {
    return (
      <>
        <LoginPage version={session.version} onLogin={handleLogin} />
        <Toast toast={toast} onClose={() => setToast(null)} />
      </>
    );
  }

  const bannedCount = tasks.filter((task) => task.status === "banned").length;
  const cloudSaver = config?.source.cloudsaver;
  const cloudReady = Boolean(
    cloudSaver &&
      cloudSaver.enable &&
      cloudSaver.server.trim() &&
      (cloudSaver.token.trim() ||
        (cloudSaver.username.trim() && cloudSaver.password.trim())),
  );
  const panReady = Boolean(
    config?.source.pansou.enable && config.source.pansou.server.trim(),
  );
  const searchReady = cloudReady || panReady;

  return (
    <TooltipProvider delayDuration={220}>
      <div className="flex min-h-screen bg-canvas">
        <Sidebar
          page={page}
          taskCount={tasks.length}
          bannedCount={bannedCount}
          username={session.username}
          version={session.version}
          mobileOpen={mobileNav}
          onNavigate={setPage}
          onLogout={handleLogout}
          onClose={() => setMobileNav(false)}
        />

        <div className="flex min-h-screen min-w-0 flex-1 flex-col">
          <Topbar
            page={page}
            search={query}
            onSearch={(value) => {
              setQuery(value);
              if (value) setPage("tasks");
            }}
            onMenu={() => setMobileNav(true)}
            onRunAll={() => void runTasks([], true)}
            onAddTask={openNewTask}
            onOpenLog={() => setPage("runs")}
          />

          <main className="flex-1 px-3 py-4 sm:px-4 lg:px-7 lg:py-5">
            <div className="mx-auto w-full max-w-[1320px]">
              {page === "overview" ? (
                <OverviewPage
                  tasks={tasks}
                  bannedCount={bannedCount}
                  latestRun={latestRun}
                  onOpenTask={openTask}
                  onRunTask={(task) =>
                    task.index != null && void runTasks([task.index], true)
                  }
                  onDeleteTask={handleDelete}
                  onRepairTask={openTask}
                  onOpenLog={openLog}
                  onRunAll={() => void runTasks([], true)}
                  onGoTasks={goTasks}
                  onGoSettings={() => setPage("settings")}
                />
              ) : null}

              {page === "tasks" ? (
                <TasksPage
                  tasks={tasks}
                  initialFilter={taskFilter}
                  query={query}
                  latestRun={latestRun}
                  onQueryChange={setQuery}
                  onOpenTask={openTask}
                  onRunTask={(task) =>
                    task.index != null && void runTasks([task.index], true)
                  }
                  onDeleteTask={handleDelete}
                  onRepairTask={openTask}
                />
              ) : null}

              {page === "files" ? <FilesPage onNotify={notify} /> : null}

              {page === "runs" ? (
                <RunsPage running={running} onOpenActive={openLog} />
              ) : null}

              {page === "settings" && config ? (
                <SettingsPage
                  config={config}
                  setConfig={updateConfig}
                  pluginStatus={pluginStatus}
                  running={running}
                  onSign={() => void runSign()}
                  onSave={handleSettingsSave}
                  onNotify={notify}
                />
              ) : null}
            </div>
          </main>
        </div>
      </div>

      <TaskSheet
        open={taskSheet.open}
        onOpenChange={(open) => setTaskSheet({ open, task: open ? taskSheet.task : null })}
        task={taskSheet.task}
        magicRegex={config?.magic_regex ?? {}}
        pluginDefaults={pluginDefaults}
        searchReady={searchReady}
        onOpenSettings={() => {
          setTaskSheet({ open: false, task: null });
          setPage("settings");
        }}
        onSave={handleSave}
        onTry={(task) => void runInline(task)}
        onDelete={handleDelete}
        onNotify={notify}
      />

      <RunLogDialog
        open={logOpen}
        onOpenChange={setLogOpen}
        lines={log}
        running={running}
        dry={dryRun || latestRun?.dry}
        onRunAgain={rerun}
      />

      <Dialog
        open={Boolean(deleteTarget)}
        onOpenChange={(open) => {
          if (!open) setDeleteTarget(null);
        }}
      >
        <DialogContent className="w-[min(420px,calc(100vw-2rem))]">
          <DialogHeader>
            <DialogTitle>删除任务</DialogTitle>
            <DialogDescription>
              删除后不会移除已经转存的文件，只会停止后续追更。
            </DialogDescription>
          </DialogHeader>
          <DialogBody>
            <div className="rounded-md border border-danger/20 bg-danger-soft/40 px-3 py-2.5 text-[13px] text-ink">
              确定删除「{deleteTarget?.taskname}」？
            </div>
          </DialogBody>
          <DialogFooter>
            <Button variant="secondary" size="sm" onClick={() => setDeleteTarget(null)}>
              取消
            </Button>
            <Button variant="danger" size="sm" onClick={() => void confirmDelete()}>
              删除任务
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Toast toast={toast} onClose={() => setToast(null)} />
    </TooltipProvider>
  );
}
