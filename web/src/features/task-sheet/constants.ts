import type { Task } from "../../lib/types";

export const emptyTask: Task = {
  taskname: "",
  shareurl: "",
  savepath: "",
  pattern: "",
  replace: "",
  magic_id: "",
  runweek: [1, 2, 3, 4, 5, 6, 7],
  runtime: ["08:00"],
  mode: "scheduled",
  addition: {},
  latest: "—",
  next_run: "—",
  status: "waiting",
  progress: { saved: 0, expected: 0, days: [0, 0, 0, 0, 0, 0, 0] },
};

export const sectionTitle =
  "text-[13px] font-semibold uppercase tracking-[0.06em] text-ink";

export const pluginFieldLabels: Record<string, string> = {
  try_match: "按任务名匹配媒体库",
  media_id: "媒体库 ID",
  auto_refresh: "自动刷新媒体库",
  mdb_name: "媒体库名称",
  mdb_dir_list: "目录列表（逗号分隔）",
  mdb_guid: "媒体库 GUID",
  dir_list: "刷新目录",
  auto_gen: "自动生成 STRM",
  enable: "启用",
  auto_clean: "解压后删除压缩包",
  auto_clean_zipdir: "删除占位目录",
  auto_download: "自动加入下载",
  download_subdir: "递归子目录",
  save_path: "下载子目录",
  pause: "添加后暂停",
  save_path_mode: "保存路径模式",
  verify_path: "验证目录",
  full_path_mode: "完整路径模式",
};

const pluginEnabledKeys: Record<string, string> = {
  auto_unarchive: "enable",
  alist_strm_gen: "auto_gen",
  alist_sync: "enable",
  aria2: "auto_download",
};

export function pluginEnabled(
  plugin: string,
  values: Record<string, string | number | boolean>,
) {
  const key = pluginEnabledKeys[plugin];
  if (key && key in values) return values[key] === true;
  return Object.values(values).some((value) => value === true);
}
