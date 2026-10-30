import type { RunView, Task } from "../types";
import { post, request } from "./client";

export function getRuns() {
  return request<{ runs: RunView[] }>("/api/runs");
}

export function getRunLog(id: string) {
  return request<{ run: RunView; lines: string[]; done: boolean }>(
    `/api/runs/${encodeURIComponent(id)}/log`,
  );
}

export function startRun(indices: number[], write: boolean, task?: Task) {
  return post<{ run_id: string }>("/api/run", {
    tasks: indices,
    write,
    ...(task ? { task } : {}),
  });
}

export function startSignin() {
  return post<{ run_id: string }>("/api/sign", {});
}

export function getLatestRun() {
  return request<RunView | null>("/api/runs/latest");
}

export function runStreamUrl(id: string) {
  return `/api/runs/${encodeURIComponent(id)}/stream`;
}
