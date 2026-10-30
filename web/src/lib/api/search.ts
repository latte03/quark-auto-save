import type { SearchResult } from "../types";
import { request } from "./client";

export function searchShares(keyword: string, deep = false) {
  return request<{ keyword: string; results: SearchResult[]; messages: string[] }>(
    `/api/search?keyword=${encodeURIComponent(keyword)}${deep ? "&deep=1" : ""}`,
  );
}

export type SourceDetection = {
  cloudsaver: { online: boolean; server?: string };
  pansou: { online: boolean; server?: string };
  alist: { online: boolean; server?: string };
};

export function detectSources() {
  return request<SourceDetection>("/api/sources/detect");
}
