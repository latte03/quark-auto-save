import { useCallback, useEffect, useRef, useState } from "react";
import {
  cancelDownload,
  clearDownloads,
  getDownloads,
  pauseDownload,
  resumeDownload,
} from "../../lib/api";
import type { DownloadJob } from "../../lib/types";
import { useToast } from "../../components/ToastProvider";

const ACTIVE_POLL_MS = 1000;
const IDLE_POLL_MS = 5000;

function isActive(job: DownloadJob) {
  return job.status === "queued" || job.status === "running";
}

export function useDownloadQueue() {
  const notify = useToast();
  const [jobs, setJobs] = useState<DownloadJob[]>([]);
  const requestSeq = useRef(0);

  const activeJobs = jobs.filter(
    (job) =>
      job.status === "queued" ||
      job.status === "running" ||
      job.status === "paused",
  );
  const finishedJobs = jobs.filter(
    (job) =>
      job.status !== "queued" &&
      job.status !== "running" &&
      job.status !== "paused",
  );
  const running = activeJobs.filter(isActive).length;
  const finishedFailed = finishedJobs.filter(
    (job) => job.status === "failed",
  ).length;
  const finishedCanceled = finishedJobs.filter(
    (job) => job.status === "canceled",
  ).length;

  // 只接受最新一次请求的结果，避免慢响应把新状态（如暂停/继续）覆盖回去；
  // 请求失败或被更新请求取代时返回 null，保留上一次的队列。
  const load = useCallback(async () => {
    const requestId = ++requestSeq.current;
    try {
      const result = await getDownloads();
      if (requestId !== requestSeq.current) return null;
      const next = result.jobs ?? [];
      setJobs(next);
      return next;
    } catch {
      return null;
    }
  }, []);

  // Poll fast while a job is running so the progress bar and speed move,
  // and back off once the queue is idle.
  useEffect(() => {
    let timer = 0;
    let stopped = false;
    const tick = async () => {
      const next = await load();
      if (stopped) return;
      const active = (next ?? []).some(isActive);
      timer = window.setTimeout(tick, active ? ACTIVE_POLL_MS : IDLE_POLL_MS);
    };
    void tick();
    return () => {
      stopped = true;
      // 卸载后仍在飞的请求不再落状态。
      requestSeq.current += 1;
      window.clearTimeout(timer);
    };
  }, [load]);

  const control = useCallback(
    async (action: (id: string) => Promise<unknown>, id: string) => {
      try {
        await action(id);
        await load();
      } catch (cause) {
        notify(cause instanceof Error ? cause.message : "操作失败", "error");
      }
    },
    [load, notify],
  );

  const clearFinished = useCallback(async () => {
    try {
      await clearDownloads();
      await load();
    } catch (cause) {
      notify(cause instanceof Error ? cause.message : "清空失败", "error");
    }
  }, [load, notify]);

  const pause = useCallback(
    (id: string) => control(pauseDownload, id),
    [control],
  );
  const resume = useCallback(
    (id: string) => control(resumeDownload, id),
    [control],
  );
  const cancel = useCallback(
    (id: string) => control(cancelDownload, id),
    [control],
  );

  return {
    jobs,
    activeJobs,
    finishedJobs,
    running,
    finishedFailed,
    finishedCanceled,
    pause,
    resume,
    cancel,
    clearFinished,
  };
}
