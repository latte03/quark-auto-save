import { useCallback, useEffect, useRef, useState } from "react";
import {
  ApiError,
  getLatestRun,
  getRunLog,
  runStreamUrl,
  startRun,
  startSignin,
} from "../lib/api";
import type { RunView, Task, TasksPayload } from "../lib/types";
import type { Notify } from "../components/ToastProvider";

const MAX_LOG_LINES = 2000;

type RunControllerParams = {
  notify: Notify;
  refreshTasks: () => Promise<TasksPayload>;
  revalidate: () => void;
};

export function useRunController({
  notify,
  refreshTasks,
  revalidate,
}: RunControllerParams) {
  const [running, setRunning] = useState(false);
  const [log, setLog] = useState<string[]>([]);
  const [logLoading, setLogLoading] = useState(false);
  const [logOpen, setLogOpen] = useState(false);
  const [dryRun, setDryRun] = useState(false);
  const [latestRun, setLatestRun] = useState<RunView | null>(null);

  const streamRef = useRef<EventSource | null>(null);
  // state 要等渲染后才更新，连点两下会同时通过 running 判断，用 ref 做同步锁。
  const runningRef = useRef(false);
  const lastRunRef = useRef<{
    indices: number[];
    write: boolean;
    task?: Task;
    sign?: boolean;
  }>({ indices: [], write: true });

  const closeStream = useCallback(() => {
    streamRef.current?.close();
    streamRef.current = null;
  }, []);

  const releaseRun = useCallback(() => {
    runningRef.current = false;
    setRunning(false);
  }, []);

  const claimRun = useCallback(() => {
    if (runningRef.current) {
      notify("已有任务在运行，等这轮结束", "warn");
      return false;
    }
    runningRef.current = true;
    setRunning(true);
    return true;
  }, [notify]);

  const loadLatest = useCallback(async () => {
    try {
      setLatestRun(await getLatestRun());
    } catch {
      // The list can stay as-is; the next run refresh will reconcile it.
    }
  }, []);

  const attach = useCallback(
    (id: string): boolean => {
      closeStream();
      setLog([]);
      runningRef.current = true;
      setRunning(true);
      setLogOpen(true);
      let source: EventSource;
      try {
        source = new EventSource(runStreamUrl(id));
      } catch (cause) {
        releaseRun();
        notify(cause instanceof Error ? cause.message : "无法连接运行日志", "error");
        return false;
      }
      streamRef.current = source;
      source.onmessage = (event) => {
        if (event.data === "[DONE]") {
          source.close();
          if (streamRef.current === source) streamRef.current = null;
          releaseRun();
          void refreshTasks()
            .catch(() => undefined)
            .finally(() => {
              void loadLatest();
            });
          return;
        }
        setLog((prev) =>
          prev.length >= MAX_LOG_LINES
            ? [...prev.slice(prev.length - MAX_LOG_LINES + 1), event.data]
            : [...prev, event.data],
        );
      };
      source.onerror = () => {
        // EventSource reconnects on its own with Last-Event-ID; only a closed
        // stream means it gave up (e.g. 401).
        if (source.readyState === EventSource.CLOSED) {
          if (streamRef.current === source) streamRef.current = null;
          releaseRun();
          revalidate();
        }
      };
      return true;
    },
    [closeStream, loadLatest, notify, refreshTasks, releaseRun, revalidate],
  );

  // 409 表示后端已经有一轮在跑：刷新任务并接管那轮的日志。
  const attachRunning = useCallback(() => {
    void refreshTasks()
      .then((payload) => {
        if (payload.running) attach(payload.running);
      })
      .catch(() => undefined);
  }, [attach, refreshTasks]);

  const reset = useCallback(() => {
    closeStream();
    releaseRun();
    setLog([]);
    setLogLoading(false);
    setLogOpen(false);
    setDryRun(false);
    setLatestRun(null);
  }, [closeStream, releaseRun]);

  const runTasks = useCallback(
    async (indices: number[], write: boolean) => {
      if (!claimRun()) return;
      lastRunRef.current = { indices, write };
      try {
        setDryRun(!write);
        const { run_id } = await startRun(indices, write);
        attach(run_id);
      } catch (cause) {
        releaseRun();
        if (cause instanceof ApiError) {
          notify(cause.message, "error");
          if (cause.status === 409) attachRunning();
          return;
        }
        notify(cause instanceof Error ? cause.message : "启动失败", "error");
      }
    },
    [attach, attachRunning, claimRun, notify, releaseRun],
  );

  const runInline = useCallback(
    async (task: Task): Promise<boolean> => {
      if (!claimRun()) return false;
      try {
        lastRunRef.current = { indices: [], write: false, task };
        setDryRun(true);
        const { run_id } = await startRun([], false, task);
        return attach(run_id);
      } catch (cause) {
        releaseRun();
        if (cause instanceof ApiError) {
          notify(cause.message, "error");
          if (cause.status === 409) attachRunning();
          return false;
        }
        notify(cause instanceof Error ? cause.message : "试运行启动失败", "error");
        return false;
      }
    },
    [attach, attachRunning, claimRun, notify, releaseRun],
  );

  const runSign = useCallback(async () => {
    if (!claimRun()) return;
    try {
      lastRunRef.current = { indices: [], write: true, sign: true };
      setDryRun(false);
      const { run_id } = await startSignin();
      attach(run_id);
    } catch (cause) {
      releaseRun();
      if (cause instanceof ApiError) {
        notify(cause.message, "error");
        if (cause.status === 409) attachRunning();
        return;
      }
      notify(cause instanceof Error ? cause.message : "签到启动失败", "error");
    }
  }, [attach, attachRunning, claimRun, notify, releaseRun]);

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
    setLogOpen(true);
    if (runningRef.current) return;
    setLog([]);
    setLogLoading(true);
    try {
      const run = await getLatestRun();
      if (runningRef.current) return;
      setLatestRun(run);
      setDryRun(Boolean(run?.dry));
      if (!run) return;
      if (!run.done) {
        attach(run.id);
        return;
      }
      // 已结束的运行没有 SSE 流可订阅，直接读落盘日志。
      const { lines } = await getRunLog(run.id);
      // 期间可能又起了新运行并接管了日志，别把它的输出覆盖掉。
      if (!runningRef.current) setLog(lines);
    } catch (cause) {
      if (!runningRef.current) {
        notify(cause instanceof Error ? cause.message : "读取运行记录失败", "error");
      }
    } finally {
      setLogLoading(false);
    }
  }, [attach, notify]);

  useEffect(
    () => () => {
      streamRef.current?.close();
    },
    [],
  );

  return {
    running,
    log,
    logLoading,
    logOpen,
    setLogOpen,
    dryRun,
    latestRun,
    attach,
    reset,
    loadLatest,
    runTasks,
    runInline,
    runSign,
    rerun,
    openLog,
  };
}
