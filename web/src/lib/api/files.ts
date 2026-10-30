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

// Keep in sync with the server's handleRenameFile limit.
const MAX_NAME_BYTES = 255;

export async function renameFile(path: string, name: string) {
  const next = name.trim();
  // Mirrors the server checks so a bad name never costs a round-trip.
  if (!next) throw new Error("新名称不能为空");
  if (next === "." || next === "..") throw new Error("新名称无效");
  if (/[\\/]/.test(next)) throw new Error("新名称不能包含路径分隔符");
  if (new TextEncoder().encode(next).length > MAX_NAME_BYTES) {
    throw new Error("新名称过长");
  }
  return post<{ fid: string; name: string }>("/api/files/rename", {
    path,
    name: next,
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
