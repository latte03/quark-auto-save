import { post } from "./client";

export type ShareValidation = {
  ok: boolean;
  title?: string;
  count?: number;
  message?: string;
};

export function validateShare(shareurl: string) {
  return post<ShareValidation>("/api/share/validate", { shareurl });
}

export type ShareDetailFile = {
  name: string;
  fid: string;
  is_dir: boolean;
  size: number;
  category?: string;
  matched: boolean;
  saved: boolean;
  renamed?: string;
};

export type ShareDetail = {
  title: string;
  savepath: string;
  pattern: string;
  replace: string;
  total: number;
  files: ShareDetailFile[];
};

export type ShareDetailRequest = {
  shareurl: string;
  pdir_fid?: string;
  taskname: string;
  pattern: string;
  replace: string;
  savepath: string;
  update_subdir?: string;
  ignore_extension?: boolean;
};

export function shareDetail(payload: ShareDetailRequest) {
  return post<ShareDetail>("/api/share/detail", payload);
}

export type RenamePreviewItem = {
  name: string;
  matched: boolean;
  renamed: string;
};

export function previewRename(payload: {
  taskname: string;
  magic_id?: string;
  pattern: string;
  replace: string;
  files: string[];
}) {
  return post<{
    pattern: string;
    replace: string;
    files: RenamePreviewItem[];
  }>("/api/rename/preview", payload);
}
