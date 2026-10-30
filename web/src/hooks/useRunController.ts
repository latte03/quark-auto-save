import { useCallback, useEffect, useRef, useState } from "react";
import {
  ApiError,
  getLatestRun,
  runStreamUrl,
  startRun,
  startSignin,
} from "../lib/api";
import type { RunView, Task, TasksPayload } from "../lib/types";
import type { Notify } from "../components/ToastProvider";

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
  const [logOpen, setLogOpen] = useState(false);
  const [dryRun, setDryRun] = useState(false);
  const [latestRun, setLatestRun] = useState<RunView | null>(null);

  const streamRef = useRef<EventSource | null>(null);
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

  const loadLatest = useCallback(async () => {
    try {
      setLatestRun(await getLatestRun());
    } catch {
      // The list can stay as-is; the next run refresh will reconcile it.
    }
  }, []);

  const attach = useCallback(
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
              void loadLatest();
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
          revalidate();
        }
      };
    },
    [closeStream, loadLatest, refreshTasks, revalidate],
  );

  const reset = useCallback(() => {
    closeStream();
    setRunning(false);
    setLog([]);
    setLogOpen(false);
    setDryRun(false);
    setLatestRun(null);
  }, [closeStream]);

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
        attach(run_id);
      } catch (cause) {
        if (cause instanceof ApiError) {
          notify(cause.message, "error");
          if (cause.status === 409) {
            void refreshTasks()
              .then((payload) => {
                if (payload.running) attach(payload.running);
              })
              .catch(() => undefined);
          }
          return;
        }
        notify(cause instanceof Error ? cause.message : "启动失败", "error");
      }
    },
    [attach, notify, refreshTasks, running],
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
        attach(run_id);
      } catch (cause) {
        notify(cause instanceof Error ? cause.message : "试运行启动失败", "error");
      }
    },
    [attach, notify, running],
  );

  const runSign = useCallback(async () => {
    if (running) {
      notify("已有任务在运行，等这轮结束", "warn");
      return;
    }
    try {
      lastRunRef.current = { indices: [], write: true, sign: true };
      const { run_id } = await startSignin();
      attach(run_id);
    } catch (cause) {
      notify(cause instanceof Error ? cause.message : "签到启动失败", "error");
    }
  }, [attach, notify, running]);

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
    if (running) return;
    try {
      const run = await getLatestRun();
      setLatestRun(run);
      setDryRun(Boolean(run?.dry));
      if (run) {
        attach(run.id);
      } else {
        setLog([]);
      }
    } catch (cause) {
      notify(cause instanceof Error ? cause.message : "读取运行记录失败", "error");
    }
  }, [attach, notify, running]);

  useEffect(
    () => () => {
      streamRef.current?.close();
    },
    [],
  );

  return {
    running,
    log,
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
