import { post, request } from "./client";

export type BrowseEntry = {
  name: string;
  fid: string;
  is_dir: boolean;
  size: number;
  category?: string;
  count?: number;
};

export function browse(path: string) {
  return request<{ path: string; fid: string; entries: BrowseEntry[] }>(
    `/api/browse?path=${encodeURIComponent(path)}`,
  );
}

export function renameFile(path: string, name: string) {
  return post<{ fid: string; name: string }>("/api/files/rename", {
    path,
    name,
  });
}

export function deleteFile(path: string) {
  return post<{ fid: string; task_id: string }>("/api/files/delete", {
    path,
    confirm: true,
  });
}

export function createFolder(path: string) {
  return post<{ path: string; fid: string }>("/api/files/mkdir", { path });
}
