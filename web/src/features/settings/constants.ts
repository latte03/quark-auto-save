import {
  Bell,
  Cookie,
  FileText,
  HardDrive,
  Palette,
  Plug,
  Search,
  Sparkles,
  type LucideIcon,
} from "lucide-react";

export type SettingsSection =
  | "appearance"
  | "cookie"
  | "logs"
  | "notify"
  | "download"
  | "plugins"
  | "magic"
  | "source";

export const sections: {
  id: SettingsSection;
  label: string;
  icon: LucideIcon;
  description: string;
}[] = [
  { id: "appearance", label: "外观", icon: Palette, description: "浅色与深色" },
  { id: "cookie", label: "Cookie", icon: Cookie, description: "账号与签到" },
  { id: "logs", label: "日志", icon: FileText, description: "运行记录" },
  { id: "notify", label: "通知", icon: Bell, description: "推送渠道" },
  { id: "download", label: "下载", icon: HardDrive, description: "本地落盘" },
  { id: "plugins", label: "插件", icon: Plug, description: "媒体库与下载" },
  { id: "magic", label: "魔法正则", icon: Sparkles, description: "命名规则" },
  { id: "source", label: "搜索源", icon: Search, description: "CloudSaver / PanSou" },
];

export const pushLabels: Record<string, string> = {
  CONSOLE: "控制台",
  HITOKOTO: "随机一言",
  QUARK_SIGN_NOTIFY: "签到结果推送",
  BARK_PUSH: "Bark 地址",
  BARK_GROUP: "Bark 分组",
  BARK_SOUND: "Bark 声音",
  BARK_ICON: "Bark 图标",
  BARK_LEVEL: "Bark 时效",
  BARK_ARCHIVE: "Bark 存档",
  BARK_URL: "Bark 跳转",
  TG_BOT_TOKEN: "Telegram Token",
  TG_USER_ID: "Telegram 用户 ID",
  TG_API_HOST: "Telegram API 地址",
  TG_PROXY_HOST: "Telegram 代理主机",
  TG_PROXY_PORT: "Telegram 代理端口",
  TG_PROXY_AUTH: "Telegram 代理认证",
  SMTP_SERVER: "SMTP 服务器",
  SMTP_SSL: "SMTP SSL",
  SMTP_EMAIL: "发件邮箱",
  SMTP_PASSWORD: "SMTP 密码",
  SMTP_NAME: "发件人姓名",
  SMTP_EMAIL_TO: "收件邮箱",
  SMTP_NAME_TO: "收件人姓名",
  PUSH_KEY: "Server酱 Key",
  DD_BOT_TOKEN: "钉钉 Token",
  DD_BOT_SECRET: "钉钉 Secret",
  FSKEY: "飞书 Key",
  GOBOT_URL: "go-cqhttp 地址",
  GOBOT_QQ: "go-cqhttp 目标",
  GOBOT_TOKEN: "go-cqhttp Token",
  GOTIFY_URL: "Gotify 地址",
  GOTIFY_TOKEN: "Gotify Token",
  GOTIFY_PRIORITY: "Gotify 优先级",
  IGOT_PUSH_KEY: "iGot Key",
  DEER_KEY: "PushDeer Key",
  DEER_URL: "PushDeer 地址",
  CHAT_URL: "群晖 Chat 地址",
  CHAT_TOKEN: "群晖 Chat Token",
  PUSH_PLUS_TOKEN: "PushPlus Token",
  PUSH_PLUS_USER: "PushPlus 群组",
  PUSH_PLUS_TEMPLATE: "PushPlus 模板",
  PUSH_PLUS_CHANNEL: "PushPlus 渠道",
  PUSH_PLUS_WEBHOOK: "PushPlus Webhook",
  PUSH_PLUS_CALLBACKURL: "PushPlus 回调",
  PUSH_PLUS_TO: "PushPlus 好友",
  QMSG_KEY: "Qmsg Key",
  QMSG_TYPE: "Qmsg 类型",
  QYWX_KEY: "企业微信机器人 Key",
  QYWX_AM: "企业微信应用",
  QYWX_ORIGIN: "企业微信代理地址",
  WE_PLUS_BOT_TOKEN: "微加机器人 Token",
  WE_PLUS_BOT_RECEIVER: "微加接收者",
  AIBOTK_KEY: "微秘书 Key",
  AIBOTK_TYPE: "微秘书类型",
  AIBOTK_NAME: "微秘书目标",
  CHRONOCAT_URL: "Chronocat 地址",
  CHRONOCAT_QQ: "Chronocat QQ",
  CHRONOCAT_TOKEN: "Chronocat Token",
  DODO_BOTTOKEN: "DoDo Token",
  DODO_BOTID: "DoDo Bot ID",
  DODO_LANDSOURCEID: "DoDo 群 ID",
  DODO_SOURCEID: "DoDo 用户 ID",
  PUSHME_KEY: "PushMe Key",
  PUSHME_URL: "PushMe 地址",
  NTFY_URL: "Ntfy 地址",
  NTFY_TOPIC: "Ntfy 主题",
  NTFY_PRIORITY: "Ntfy 优先级",
  NTFY_TOKEN: "Ntfy Token",
  NTFY_USERNAME: "Ntfy 用户名",
  NTFY_PASSWORD: "Ntfy 密码",
  NTFY_ACTIONS: "Ntfy 动作",
  WXPUSHER_APP_TOKEN: "WxPusher Token",
  WXPUSHER_TOPIC_IDS: "WxPusher 主题",
  WXPUSHER_UIDS: "WxPusher 用户",
  WEBHOOK_URL: "Webhook 地址",
  WEBHOOK_METHOD: "Webhook 方法",
  WEBHOOK_CONTENT_TYPE: "Webhook 类型",
  WEBHOOK_BODY: "Webhook 请求体",
  WEBHOOK_HEADERS: "Webhook 请求头",
};

export const pluginFieldLabels: Record<string, string> = {
  url: "服务器地址",
  token: "Token",
  base_url: "服务地址",
  app_name: "应用名",
  username: "用户名",
  password: "密码",
  secret_string: "密钥字符串",
  api_key: "API Key",
  secret: "Secret",
  host_port: "RPC 地址",
  dir: "下载目录",
  storage_id: "存储 ID",
  strm_save_dir: "STRM 保存目录",
  strm_replace_host: "替换主机",
  quark_storage_id: "夸克存储 ID",
  save_storage_id: "保存存储 ID",
  tv_mode: "TV 模式",
  webhook: "Webhook 地址",
  strmtask: "SmartStrm 任务",
  xlist_path_fix: "路径映射",
  global_enable: "全局启用",
  max_concurrent: "并发数",
  quark_root_path: "夸克根路径",
  config_id: "配置 ID",
  cookie: "Cookie",
};

export const magicVariables: { name: string; meaning: string }[] = [
  { name: "{TASKNAME}", meaning: "任务名称" },
  { name: "{CHINESE}", meaning: "文件名里的中文片段" },
  { name: "{EXT}", meaning: "扩展名" },
  { name: "{DATE}", meaning: "日期，不足 8 位用当年补齐" },
  { name: "{YEAR}", meaning: "四位年份" },
  { name: "{S}", meaning: "季号数字，如 S01 里的 01" },
  { name: "{SXX}", meaning: "带 S 的季号，如 S01；缺失时为 S01" },
  { name: "{E}", meaning: "集数" },
  { name: "{PART}", meaning: "上 / 中 / 下、一 / 二 / 三" },
  { name: "{VER}", meaning: "「XX版」里的版本标记" },
  { name: "{I} / {I+}", meaning: "序号；带 + 会接着目标目录已有编号续排" },
];

export function newRuleId() {
  if (typeof crypto !== "undefined" && "randomUUID" in crypto) {
    return `m${crypto.randomUUID().replaceAll("-", "").slice(0, 8)}`;
  }
  return `m${Math.random().toString(36).slice(2, 10)}`;
}

export function isSecret(key: string) {
  return /token|secret|password|key|cookie/i.test(key);
}

export function formatFree(bytes: number) {
  if (!bytes) return "未知";
  const gb = bytes / 1024 ** 3;
  return gb >= 1024 ? `${(gb / 1024).toFixed(1)} TB` : `${gb.toFixed(1)} GB`;
}
