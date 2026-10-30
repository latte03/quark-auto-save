import type { DownloadJob } from "../types";
import { post, request } from "./client";

export function getDownloads() {
  return request<{ jobs: DownloadJob[] }>("/api/downloads");
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

export function clearDownloads() {
  return post<{ removed: number }>("/api/downloads/clear", {});
}
