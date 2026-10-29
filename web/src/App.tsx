import {
  lazy,
  Suspense,
  useCallback,
  useEffect,
  useRef,
  useState,
  type CSSProperties,
  type SetStateAction,
} from "react";
import { CircleAlert, Loader2 } from "lucide-react";
import { AppSidebar } from "./components/layout/AppSidebar";
import { Topbar } from "./components/layout/Topbar";
import { Toast, type ToastData, type ToastTone } from "./components/Toast";
import { ConfirmDialog } from "./components/ConfirmDialog";
import { deepEqual } from "./lib/compare";
import { useBeforeUnload } from "./hooks/use-before-unload";
import { applyTheme, readThemeMode, saveThemeMode, type ThemeMode } from "./lib/theme";
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
import { ScrollArea } from "./components/ui/scroll-area";
import { SidebarInset, SidebarProvider } from "./components/ui/sidebar";
import { TooltipProvider } from "./components/ui/tooltip";
import { OverviewPage } from "./features/OverviewPage";
import type { TaskFilter } from "./features/TasksPage";
import { LoginPage } from "./features/LoginPage";

// Everything except the landing page loads on first visit, so the initial
// bundle stays close to the shell.
const TasksPage = lazy(() =>
  import("./features/TasksPage").then((module) => ({ default: module.TasksPage })),
);
const FilesPage = lazy(() =>
  import("./features/FilesPage").then((module) => ({ default: module.FilesPage })),
);
const RunsPage = lazy(() =>
  import("./features/RunsPage").then((module) => ({ default: module.RunsPage })),
);
const SettingsPage = lazy(() =>
  import("./features/SettingsPage").then((module) => ({ default: module.SettingsPage })),
);
const TaskSheet = lazy(() =>
  import("./features/TaskSheet").then((module) => ({ default: module.TaskSheet })),
);
const RunLogDialog = lazy(() =>
  import("./features/RunLogDialog").then((module) => ({ default: module.RunLogDialog })),
);
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
  setUnauthorizedHandler,
  startRun,
  startSignin,
  updateTask,
  type SessionInfo,
} from "./lib/api";
import type { AppConfig, Page, PluginConfig, RunView, Task, TasksPayload } from "./lib/types";

export default function App() {
  const [session, setSession] = useState<SessionInfo | null>(null);
  const [bootError, setBootError] = useState("");
  const [expiredNotice, setExpiredNotice] = useState("");
  const [theme, setTheme] = useState<ThemeMode>(readThemeMode);
  const [config, setConfig] = useState<AppConfig | null>(null);
  const [savedConfig, setSavedConfig] = useState<AppConfig | null>(null);
  const [tasks, setTasks] = useState<Task[]>([]);
  const [pluginDefaults, setPluginDefaults] = useState<Record<string, PluginConfig>>({});
  const [pluginStatus, setPluginStatus] = useState<Record<string, boolean>>({});
  const [latestRun, setLatestRun] = useState<RunView | null>(null);

  const [page, setPage] = useState<Page>("overview");
  const [leaveTarget, setLeaveTarget] = useState<Page | null>(null);
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
  const contentRef = useRef<HTMLDivElement | null>(null);
  const lastRunRef = useRef<{
    indices: number[];
    write: boolean;
    task?: Task;
    sign?: boolean;
  }>({ indices: [], write: true });
  const booted = useRef(false);
  const checkingSession = useRef(false);

  const notify = useCallback((message: string, tone: ToastTone = "info") => {
    setToast({ id: Date.now(), message, tone });
    if (toastTimer.current) window.clearTimeout(toastTimer.current);
    toastTimer.current = null;
    // Failures stay until dismissed; success and hints fade on their own.
    if (tone !== "error") {
      toastTimer.current = window.setTimeout(() => setToast(null), 3200);
    }
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

  const dropSession = useCallback(
    (notice: string) => {
      closeStream();
      setRunning(false);
      setLogOpen(false);
      setTaskSheet({ open: false, task: null });
      setConfig(null);
      setTasks([]);
      setLatestRun(null);
      setExpiredNotice(notice);
      setSession((prev) => ({
        authenticated: false,
        username: prev?.username ?? "",
        version: prev?.version ?? "",
      }));
    },
    [closeStream],
  );

  // Any 401 — REST or the run stream — means the cookie is gone; ask the
  // server once, then fall back to the login form instead of a dead console.
  const revalidateSession = useCallback(() => {
    if (checkingSession.current) return;
    checkingSession.current = true;
    void getSession()
      .then((info) => {
        if (!info.authenticated) dropSession("登录状态已过期，请重新登录");
      })
      .catch(() => undefined)
      .finally(() => {
        window.setTimeout(() => {
          checkingSession.current = false;
        }, 1000);
      });
  }, [dropSession]);

  useEffect(() => {
    setUnauthorizedHandler(revalidateSession);
    return () => setUnauthorizedHandler(null);
  }, [revalidateSession]);

  useEffect(() => {
    applyTheme(theme);
  }, [theme]);

  useEffect(() => {
    if (theme !== "system") return;
    const query = window.matchMedia("(prefers-color-scheme: dark)");
    const onChange = () => applyTheme("system", query.matches);
    query.addEventListener("change", onChange);
    return () => query.removeEventListener("change", onChange);
  }, [theme]);

  const changeTheme = useCallback((mode: ThemeMode) => {
    saveThemeMode(mode);
    setTheme(mode);
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
          revalidateSession();
        }
      };
    },
    [closeStream, refreshTasks, revalidateSession],
  );

  const bootstrap = useCallback(async () => {
    setBootError("");
    try {
      const info = await getSession();
      setSession(info);
      if (!info.authenticated) return;
      const [cfg, payload] = await Promise.all([getConfig(), getTasks()]);
      setConfig(cfg);
      setSavedConfig(cfg);
      applyTasks(payload);
      void getLatestRun()
        .then((run) => setLatestRun(run))
        .catch(() => undefined);
      if (payload.running) attachRun(payload.running);
    } catch (cause) {
      if (cause instanceof ApiError && cause.status === 401) {
        dropSession("登录状态已过期，请重新登录");
        return;
      }
      // Keep the shell in its loading state so the retry screen can come back.
      setBootError(cause instanceof Error ? cause.message : "加载失败");
    }
  }, [applyTasks, attachRun, dropSession]);

  useEffect(() => {
    if (booted.current) return;
    booted.current = true;
    void bootstrap();
  }, [bootstrap]);

  useEffect(() => {
    const el = contentRef.current;
    if (!el) return;
    const viewport = el.closest("[data-slot=scroll-area-viewport]");
    if (viewport instanceof HTMLElement) viewport.scrollTop = 0;
    el.classList.remove("page-enter");
    void el.offsetWidth;
    el.classList.add("page-enter");
  }, [page]);

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
          notify(cause.message, "error");
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
        notify(cause instanceof Error ? cause.message : "启动失败", "error");
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
        notify(cause instanceof Error ? cause.message : "试运行启动失败", "error");
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
      notify(cause instanceof Error ? cause.message : "签到启动失败", "error");
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
      notify(cause instanceof Error ? cause.message : "读取运行记录失败", "error");
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
      notify(cause instanceof Error ? cause.message : "保存失败", "error");
    }
  };

  const handleDelete = (task: Task) => setDeleteTarget(task);

  const confirmDelete = async () => {
    if (!deleteTarget || deleteTarget.index == null) return;
    try {
      const payload = await deleteTask(deleteTarget.index);
      setTasks(payload.tasks);
      setDeleteTarget(null);
      notify("任务已删除", "ok");
    } catch (cause) {
      notify(cause instanceof Error ? cause.message : "删除失败", "error");
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
    setSavedConfig(saved);
    await refreshTasks();
  };

  // Values the settings page fills in on its own (download dir, detected
  // sources) are not unsaved edits, so the baseline absorbs them.
  const [adoptTick, setAdoptTick] = useState(0);
  const adoptedTick = useRef(0);
  const adoptDefaults = useCallback(() => setAdoptTick((tick) => tick + 1), []);

  useEffect(() => {
    if (adoptTick === adoptedTick.current) return;
    adoptedTick.current = adoptTick;
    setSavedConfig(config);
  }, [adoptTick, config]);

  const updateConfig = useCallback((updater: SetStateAction<AppConfig>) => {
    setConfig((prev) => {
      if (!prev) return prev;
      return typeof updater === "function" ? updater(prev) : updater;
    });
  }, []);

  const settingsDirty = Boolean(
    page === "settings" && config && savedConfig && !deepEqual(config, savedConfig),
  );
  useBeforeUnload(settingsDirty);

  // Leaving settings with edits asks first; the draft is thrown away, not saved.
  const navigate = useCallback(
    (next: Page) => {
      if (next === page) return;
      if (settingsDirty) {
        setLeaveTarget(next);
        return;
      }
      setPage(next);
    },
    [page, settingsDirty],
  );

  const goTasks = (filter: TaskFilter = "all") => {
    setTaskFilter(filter);
    navigate("tasks");
  };

  if (!session) {
    return (
      <div className="grid min-h-screen place-items-center bg-canvas px-4">
        {bootError ? (
          <div
            role="alert"
            className="flex w-full max-w-[420px] flex-col items-start gap-3 rounded-panel border border-danger/25 bg-panel px-4 py-3.5 shadow-panel"
          >
            <div className="flex items-center gap-2 text-[13px] font-medium text-ink">
              <CircleAlert className="h-4 w-4 text-danger" />
              无法连接控制台
            </div>
            <p className="text-[12px] leading-5 text-muted-ink">{bootError}</p>
            <Button
              size="sm"
              variant="secondary"
              onClick={() => {
                void bootstrap();
              }}
            >
              重试
            </Button>
          </div>
        ) : (
          <div className="flex items-center gap-2 text-[12px] text-muted-ink">
            <Loader2 className="h-4 w-4 animate-spin" />
            正在连接控制台…
          </div>
        )}
      </div>
    );
  }

  if (!session.authenticated) {
    return (
      <>
        <LoginPage
          version={session.version}
          notice={expiredNotice}
          onLogin={handleLogin}
        />
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
    <TooltipProvider delay={220}>
      <SidebarProvider
        className="h-svh overflow-hidden bg-canvas"
        style={{ "--sidebar-width": "232px" } as CSSProperties}
      >
        <AppSidebar
          page={page}
          taskCount={tasks.length}
          bannedCount={bannedCount}
          username={session.username}
          version={session.version}
          onNavigate={navigate}
          onLogout={handleLogout}
        />

        <SidebarInset className="h-svh min-w-0 flex-col overflow-hidden bg-canvas">
          <Topbar
            search={page === "tasks" ? query : null}
            onSearch={setQuery}
            onRunAll={() => void runTasks([], true)}
            onAddTask={openNewTask}
            onOpenLog={() => navigate("runs")}
          />

          <ScrollArea className="min-h-0 flex-1">
            <div className="px-3 py-4 sm:px-4 lg:px-7 lg:py-5">
              <div ref={contentRef} className="mx-auto w-full max-w-[1320px]">
                <Suspense
                  fallback={
                    <div className="flex h-48 items-center justify-center text-muted-ink">
                      <Loader2 className="h-4 w-4 animate-spin" />
                    </div>
                  }
                >
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
                    dirty={settingsDirty}
                    running={running}
                    theme={theme}
                    onThemeChange={changeTheme}
                    onSign={() => void runSign()}
                    onSave={handleSettingsSave}
                    onTasksReload={() => void refreshTasks()}
                    onNotify={notify}
                    onAdoptDefaults={adoptDefaults}
                  />
                ) : null}
                </Suspense>
              </div>
            </div>
          </ScrollArea>
        </SidebarInset>
      </SidebarProvider>

      <Suspense fallback={null}>
      <TaskSheet
        open={taskSheet.open}
        onOpenChange={(open) => setTaskSheet({ open, task: open ? taskSheet.task : null })}
        task={taskSheet.task}
        magicRegex={config?.magic_regex ?? []}
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
      </Suspense>

      <Suspense fallback={null}>
        <RunLogDialog
          open={logOpen}
        onOpenChange={setLogOpen}
        lines={log}
        running={running}
        dry={dryRun || latestRun?.dry}
          onRunAgain={rerun}
        />
      </Suspense>

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
            <Button variant="destructive" size="sm" onClick={() => void confirmDelete()}>
              删除任务
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <ConfirmDialog
        open={leaveTarget !== null}
        onOpenChange={(open) => {
          if (!open) setLeaveTarget(null);
        }}
        title="放弃未保存的设置？"
        description="设置里有改动还没保存，离开后这些改动会丢失。"
        confirmLabel="放弃更改"
        cancelLabel="继续编辑"
        onConfirm={() => {
          const target = leaveTarget;
          setLeaveTarget(null);
          if (savedConfig) setConfig(savedConfig);
          if (target) setPage(target);
        }}
      />

      <Toast
        toast={toast}
        onClose={() => setToast(null)}
        className={page === "settings" ? "bottom-[72px]" : undefined}
      />
    </TooltipProvider>
  );
}
