import {
  lazy,
  Suspense,
  useCallback,
  useEffect,
  useRef,
  useState,
  type CSSProperties,
} from "react";
import { Loader2 } from "lucide-react";
import { AppSidebar } from "./components/layout/AppSidebar";
import { Topbar } from "./components/layout/Topbar";
import { BootScreen } from "./components/layout/BootScreen";
import { Toast } from "./components/Toast";
import { ToastProvider } from "./components/ToastProvider";
import { ConfirmDialog } from "./components/ConfirmDialog";
import { useBeforeUnload } from "./hooks/use-before-unload";
import { useConsoleSession } from "./hooks/useConsoleSession";
import { useRunController } from "./hooks/useRunController";
import { useSettingsDraft } from "./hooks/useSettingsDraft";
import { useTaskCollection } from "./hooks/useTaskCollection";
import { useThemeMode } from "./hooks/useThemeMode";
import { useToastQueue } from "./hooks/useToastQueue";
import { ScrollArea } from "./components/ui/scroll-area";
import { SidebarInset, SidebarProvider } from "./components/ui/sidebar";
import { TooltipProvider } from "./components/ui/tooltip";
import { OverviewPage } from "./features/OverviewPage";
import type { TaskFilter } from "./features/TasksPage";
import { LoginPage } from "./features/LoginPage";
import { DeleteTaskDialog } from "./features/DeleteTaskDialog";

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

import { ApiError, getConfig, getSession, getTasks, login } from "./lib/api";
import type { Page, Task } from "./lib/types";

export default function App() {
  const { toast, notify, dismiss } = useToastQueue();
  const { theme, changeTheme } = useThemeMode();
  const {
    session,
    setSession,
    bootError,
    setBootError,
    expiredNotice,
    markExpired,
    revalidate,
    signOut,
  } = useConsoleSession();

  const [page, setPage] = useState<Page>("overview");
  const [leaveTarget, setLeaveTarget] = useState<Page | null>(null);
  const [taskFilter, setTaskFilter] = useState<TaskFilter>("all");
  const [query, setQuery] = useState("");
  const [taskSheet, setTaskSheet] = useState<{ open: boolean; task: Task | null }>({
    open: false,
    task: null,
  });
  const [deleteTarget, setDeleteTarget] = useState<Task | null>(null);
  const [deleting, setDeleting] = useState(false);

  const taskStore = useTaskCollection(Boolean(session?.authenticated) && page === "tasks");
  const runs = useRunController({
    notify,
    refreshTasks: taskStore.refresh,
    revalidate,
  });
  const settings = useSettingsDraft();
  const config = settings.config;

  const { apply: applyTasks, refresh: refreshTasks, save: saveTask, remove: removeTask } =
    taskStore;
  const { runTasks, runInline, runSign, attach: attachRun, loadLatest: loadLatestRun } = runs;
  const { load: loadConfig, clear: clearConfig } = settings;

  const booted = useRef(false);
  // 重试/登录可能触发多次引导：只采用最新一次的结果，旧请求晚到也不覆盖状态。
  const bootSeq = useRef(0);
  const contentRef = useRef<HTMLDivElement | null>(null);

  const bootstrap = useCallback(async () => {
    const runId = ++bootSeq.current;
    const current = () => runId === bootSeq.current;
    setBootError("");
    try {
      const info = await getSession();
      if (!current()) return;
      if (!info.authenticated) {
        setSession(info);
        return;
      }
      // 先取到配置与任务再标记已登录，失败时才回得到带重试的启动页。
      const [cfg, payload] = await Promise.all([getConfig(), getTasks()]);
      if (!current()) return;
      loadConfig(cfg);
      applyTasks(payload);
      setSession(info);
      void loadLatestRun();
      if (payload.running) attachRun(payload.running);
    } catch (cause) {
      if (!current()) return;
      if (cause instanceof ApiError && cause.status === 401) {
        markExpired();
        return;
      }
      // Keep the shell in its loading state so the retry screen can come back.
      setBootError(cause instanceof Error ? cause.message : "加载失败");
    }
  }, [
    applyTasks,
    attachRun,
    loadConfig,
    loadLatestRun,
    markExpired,
    setBootError,
    setSession,
  ]);

  useEffect(() => {
    if (booted.current) return;
    booted.current = true;
    void bootstrap();
  }, [bootstrap]);

  // Expiring or logging out must drop every piece of per-session state.
  const wasAuthed = useRef(false);
  const resetRuns = runs.reset;
  const clearTasks = taskStore.clear;
  useEffect(() => {
    if (session?.authenticated) {
      wasAuthed.current = true;
      return;
    }
    if (!wasAuthed.current) return;
    wasAuthed.current = false;
    resetRuns();
    clearTasks();
    clearConfig();
    setTaskSheet({ open: false, task: null });
    setDeleteTarget(null);
    setDeleting(false);
    setLeaveTarget(null);
    setPage("overview");
    setTaskFilter("all");
    setQuery("");
  }, [clearConfig, clearTasks, resetRuns, session?.authenticated]);

  useEffect(() => {
    const el = contentRef.current;
    if (!el) return;
    const viewport = el.closest("[data-slot=scroll-area-viewport]");
    if (viewport instanceof HTMLElement) viewport.scrollTop = 0;
    el.classList.remove("page-enter");
    void el.offsetWidth;
    el.classList.add("page-enter");
  }, [page]);

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

  const handleLogout = () => {
    void signOut();
  };

  const openTask = (task: Task) => setTaskSheet({ open: true, task });
  const openNewTask = () => setTaskSheet({ open: true, task: null });
  const handleDelete = (task: Task) => setDeleteTarget(task);

  const handleSave = async (next: Task, runAfter: boolean): Promise<boolean> => {
    const previous = taskSheet.task;
    let savedIndex: number | null = null;
    try {
      const { index } = await saveTask(previous, next);
      savedIndex = index;
      notify(runAfter ? "任务已保存，开始运行" : "任务已保存", "ok");
    } catch (cause) {
      notify(cause instanceof Error ? cause.message : "保存失败", "error");
      return false;
    }
    if (!runAfter) return true;
    // 启动失败属于运行问题，不能再报成「保存失败」。
    if (savedIndex == null) {
      notify("任务已保存，但没能定位到它，请在列表里手动运行", "warn");
      return true;
    }
    await runTasks([savedIndex], true);
    return true;
  };

  const confirmDelete = async () => {
    if (!deleteTarget || deleting) return;
    setDeleting(true);
    try {
      await removeTask(deleteTarget);
      setDeleteTarget(null);
      // 从任务面板发起删除时，成功后一并收起面板。
      setTaskSheet({ open: false, task: null });
      notify("任务已删除", "ok");
    } catch (cause) {
      notify(cause instanceof Error ? cause.message : "删除失败", "error");
    } finally {
      setDeleting(false);
    }
  };

  const handleSettingsSave = async () => {
    await settings.save();
    // 配置已经落盘；任务列表刷新失败不该把这次保存报成失败。
    void refreshTasks().catch(() => undefined);
  };

  const settingsDirty = page === "settings" && settings.dirty;
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
    return <BootScreen error={bootError} onRetry={() => void bootstrap()} />;
  }

  if (!session.authenticated) {
    return (
      <>
        <LoginPage
          version={session.version}
          notice={expiredNotice || bootError}
          onLogin={handleLogin}
        />
        <Toast toast={toast} onClose={dismiss} />
      </>
    );
  }

  const tasks = taskStore.tasks;
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
      <ToastProvider notify={notify}>
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
                        latestRun={runs.latestRun}
                        onOpenTask={openTask}
                        onRunTask={(task) =>
                          task.index != null && void runTasks([task.index], true)
                        }
                        onDeleteTask={handleDelete}
                        onRepairTask={openTask}
                        onOpenLog={() => void runs.openLog()}
                        onRunAll={() => void runTasks([], true)}
                        onGoTasks={goTasks}
                      />
                    ) : null}

                    {page === "tasks" ? (
                      <TasksPage
                        tasks={tasks}
                        initialFilter={taskFilter}
                        query={query}
                        latestRun={runs.latestRun}
                        onQueryChange={setQuery}
                        onOpenTask={openTask}
                        onRunTask={(task) =>
                          task.index != null && void runTasks([task.index], true)
                        }
                        onDeleteTask={handleDelete}
                        onRepairTask={openTask}
                      />
                    ) : null}

                    {page === "files" ? <FilesPage /> : null}

                    {page === "runs" ? (
                      <RunsPage
                        running={runs.running}
                        onOpenActive={() => void runs.openLog()}
                      />
                    ) : null}

                    {page === "settings" && config ? (
                      <SettingsPage
                        config={config}
                        setConfig={settings.update}
                        pluginStatus={taskStore.pluginStatus}
                        dirty={settingsDirty}
                        running={runs.running}
                        theme={theme}
                        onThemeChange={changeTheme}
                        onSign={() => void runSign()}
                        onSave={handleSettingsSave}
                        onTasksReload={() => void refreshTasks()}
                        onAdoptDefaults={settings.adoptDefaults}
                        onAdoptPatch={settings.adoptPatch}
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
            onOpenChange={(open) =>
              setTaskSheet({ open, task: open ? taskSheet.task : null })
            }
            task={taskSheet.task}
            magicRegex={config?.magic_regex ?? []}
            pluginDefaults={taskStore.pluginDefaults}
            searchReady={searchReady}
            onOpenSettings={() => {
              setTaskSheet({ open: false, task: null });
              setPage("settings");
            }}
            onSave={handleSave}
            onTry={runInline}
            onDelete={handleDelete}
          />
        </Suspense>

        <Suspense fallback={null}>
          <RunLogDialog
            open={runs.logOpen}
            onOpenChange={runs.setLogOpen}
            lines={runs.log}
            loading={runs.logLoading}
            running={runs.running}
            dry={runs.dryRun || runs.latestRun?.dry}
            onRunAgain={runs.rerun}
          />
        </Suspense>

        <DeleteTaskDialog
          target={deleteTarget}
          busy={deleting}
          onOpenChange={() => setDeleteTarget(null)}
          onConfirm={() => void confirmDelete()}
        />

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
            if (settings.savedConfig) settings.update(settings.savedConfig);
            if (target) setPage(target);
          }}
        />
      </ToastProvider>

      <Toast
        toast={toast}
        onClose={dismiss}
        className={page === "settings" ? "bottom-[72px]" : undefined}
      />
    </TooltipProvider>
  );
}
