export const weekLabels = ["一", "二", "三", "四", "五", "六", "日"];

export const pluginLabels: Record<string, string> = {
  emby: "Emby 媒体库",
  plex: "Plex 媒体库",
  fnv: "飞牛影视 v1",
  fnv_refresh_v2: "飞牛影视 v2",
  alist: "Alist 刷新",
  alist_strm_gen: "Alist STRM 生成",
  alist_strm: "alist-strm 触发",
  alist_sync: "Alist 跨盘同步",
  aria2: "Aria2 下载",
  auto_unarchive: "自动云解压",
  smartstrm: "SmartStrm Webhook",
};

export const pluginDescriptions: Record<string, string> = {
  emby: "按任务名匹配媒体库，转存后刷新对应剧集。",
  plex: "按保存路径匹配 Plex 媒体库位置并刷新。",
  fnv: "飞牛影视 v1，HTTP + cse 签名，支持局部目录刷新。",
  fnv_refresh_v2: "飞牛影视 v2，Nuitka 编译插件，支持 fnOS 长 token。",
  alist: "刷新 Alist / OpenList 中对应的夸克目录。",
  alist_strm_gen: "递归生成 .strm 文件到本地媒体目录。",
  alist_strm: "触发外部 alist-strm 的指定配置。",
  alist_sync: "调用 Alist 跨网盘复制，支持 TV 模式去重。",
  aria2: "把新转存文件加入 Aria2 下载队列。",
  auto_unarchive: "云端解压 zip / rar / 7z，并整理解压目录。",
  smartstrm: "通过 webhook 触发 SmartStrm 生成任务。",
};

export function formatBytes(size: number) {
  if (!size) return "—";
  const units = ["B", "KB", "MB", "GB", "TB"];
  let value = size;
  let index = 0;
  while (value >= 1024 && index < units.length - 1) {
    value /= 1024;
    index += 1;
  }
  return `${value.toFixed(value >= 100 ? 0 : 1)} ${units[index]}`;
}
