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

export function useDownloadQueue() {
  const notify = useToast();
  const [jobs, setJobs] = useState<DownloadJob[]>([]);
  const activeRef = useRef(false);

  const activeJobs = jobs.filter(
    (job) =>
      job.status === "queued" ||
      job.status === "running" ||
      job.status === "paused",
  );
  const finishedJobs = jobs.filter((job) => !activeJobs.includes(job));
  const running = activeJobs.filter(
    (job) => job.status === "queued" || job.status === "running",
  ).length;
  const finishedFailed = finishedJobs.filter(
    (job) => job.status === "failed",
  ).length;
  const finishedCanceled = finishedJobs.filter(
    (job) => job.status === "canceled",
  ).length;

  activeRef.current = running > 0;

  // Poll fast while a job is running so the progress bar and speed move,
  // and back off once the queue is idle.
  useEffect(() => {
    let alive = true;
    let timer = 0;
    const tick = async () => {
      try {
        const result = await getDownloads();
        if (alive) setJobs(result.jobs ?? []);
      } catch {
        if (alive) setJobs([]);
      }
      if (alive) {
        timer = window.setTimeout(tick, activeRef.current ? 1000 : 5000);
      }
    };
    void tick();
    return () => {
      alive = false;
      window.clearTimeout(timer);
    };
  }, []);

  const control = useCallback(
    async (action: (id: string) => Promise<unknown>, id: string) => {
      try {
        await action(id);
        const result = await getDownloads();
        setJobs(result.jobs ?? []);
      } catch (cause) {
        notify(cause instanceof Error ? cause.message : "操作失败", "error");
      }
    },
    [notify],
  );

  const clearFinished = useCallback(async () => {
    try {
      await clearDownloads();
      const result = await getDownloads();
      setJobs(result.jobs ?? []);
    } catch (cause) {
      notify(cause instanceof Error ? cause.message : "清空失败", "error");
    }
  }, [notify]);

  return {
    jobs,
    activeJobs,
    finishedJobs,
    running,
    finishedFailed,
    finishedCanceled,
    pause: (id: string) => control(pauseDownload, id),
    resume: (id: string) => control(resumeDownload, id),
    cancel: (id: string) => control(cancelDownload, id),
    clearFinished,
  };
}
