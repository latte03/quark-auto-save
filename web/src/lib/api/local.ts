import { post, request } from "./client";

export type LocalDirCheck = {
  ok: boolean;
  writable: boolean;
  free_bytes: number;
  message: string;
};

export function validateLocalDir(path: string) {
  return post<LocalDirCheck>("/api/local/validate", { path });
}

export function getAuthorizedDirs() {
  return request<{ paths: string[] }>("/api/local/authorized");
}

export type LocalDirBrowse = {
  roots: string[];
  path: string;
  parent: string;
  dirs: { name: string; path: string }[];
};

export function browseLocalDirs(path?: string) {
  const query = path ? `?path=${encodeURIComponent(path)}` : "";
  return request<LocalDirBrowse>(`/api/local/browse${query}`);
}
