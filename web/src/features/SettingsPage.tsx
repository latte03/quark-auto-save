import { useEffect, useRef, useState } from "react";
import {
  AlertTriangle,
  Bell,
  CalendarClock,
  CheckCircle2,
  ChevronDown,
  Cookie,
  FolderOpen,
  HardDrive,
  KeyRound,
  Loader2,
  Plug,
  Plus,
  Radar,
  Save,
  Search,
  Sparkles,
  Trash2,
  Zap,
} from "lucide-react";
import type { AppConfig, MagicRegex, PluginConfig } from "../lib/types";
import { pluginDescriptions, pluginLabels } from "../lib/mock";
import {
  detectSources,
  getAuthorizedDirs,
  previewRename,
  searchShares,
  testNotify,
  validateLocalDir,
  type LocalDirCheck,
  type SourceDetection,
} from "../lib/api";
import { pickSharedDirectory } from "../lib/trim";
import { LocalDirDialog } from "./LocalDirDialog";
import { cn } from "../lib/utils";
import { Badge } from "../components/ui/badge";
import { Button } from "../components/ui/button";
import { Field, SwitchRow } from "../components/ui/field";
import { Input, Textarea } from "../components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "../components/ui/select";
import { PageSection } from "../components/PageSection";

type SettingsSection =
  | "cookie"
  | "schedule"
  | "notify"
  | "download"
  | "plugins"
  | "magic"
  | "source";

const sections: {
  id: SettingsSection;
  label: string;
  icon: typeof Cookie;
  description: string;
}[] = [
  { id: "cookie", label: "Cookie", icon: Cookie, description: "账号与签到" },
  { id: "schedule", label: "定时", icon: CalendarClock, description: "Crontab" },
  { id: "notify", label: "通知", icon: Bell, description: "推送渠道" },
  { id: "download", label: "下载", icon: HardDrive, description: "本地落盘" },
  { id: "plugins", label: "插件", icon: Plug, description: "媒体库与下载" },
  { id: "magic", label: "魔法正则", icon: Sparkles, description: "命名规则" },
  { id: "source", label: "搜索源", icon: Search, description: "CloudSaver / PanSou" },
];

const pushLabels: Record<string, string> = {
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

const pluginFieldLabels: Record<string, string> = {
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

function isSecret(key: string) {
  return /token|secret|password|key|cookie/i.test(key);
}

function formatFree(bytes: number) {
  if (!bytes) return "未知";
  const gb = bytes / 1024 ** 3;
  return gb >= 1024 ? `${(gb / 1024).toFixed(1)} TB` : `${gb.toFixed(1)} GB`;
}

function fieldLabel(key: string) {
  return pluginFieldLabels[key] ?? key;
}

function PluginFields({
  plugin,
  values,
  onChange,
}: {
  plugin: string;
  values: PluginConfig;
  onChange: (key: string, value: string | number | boolean) => void;
}) {
  return (
    <div className="grid gap-x-3 md:grid-cols-2">
      {Object.entries(values).map(([key, value]) => {
        if (typeof value === "boolean") {
          return (
            <div key={key} className="md:col-span-1">
              <SwitchRow
                label={fieldLabel(key)}
                checked={value}
                onCheckedChange={(checked) => onChange(key, checked)}
              />
            </div>
          );
        }
        if (typeof value === "number") {
          return (
            <Field key={key} label={fieldLabel(key)} className="py-1.5">
              <Input
                type="number"
                value={value}
                onChange={(event) => onChange(key, Number(event.target.value))}
              />
            </Field>
          );
        }
        return (
          <Field key={key} label={fieldLabel(key)} className="py-1.5">
            <Input
              type={isSecret(key) ? "password" : "text"}
              value={value}
              placeholder={isSecret(key) ? "已配置，修改后保存" : `${plugin} ${key}`}
              onChange={(event) => onChange(key, event.target.value)}
            />
          </Field>
        );
      })}
    </div>
  );
}

type SettingsPageProps = {
  config: AppConfig;
  setConfig: (updater: React.SetStateAction<AppConfig>) => void;
  pluginStatus: Record<string, boolean>;
  running: boolean;
  onSign: () => void;
  onSave: () => Promise<void>;
  onNotify: (message: string, tone?: "info" | "ok" | "warn") => void;
};

export function SettingsPage({
  config,
  setConfig,
  pluginStatus,
  running,
  onSign,
  onSave,
  onNotify,
}: SettingsPageProps) {
  const [section, setSection] = useState<SettingsSection>("cookie");
  const [openPlugin, setOpenPlugin] = useState<string | null>(
    Object.keys(config.plugins)[0] ?? null,
  );
  const [sample, setSample] = useState("Black.Mirror.S07E05.mkv");
  const [newChannel, setNewChannel] = useState("");
  const [saving, setSaving] = useState(false);
  const [testingSearch, setTestingSearch] = useState(false);
  const [testingNotify, setTestingNotify] = useState(false);
  const [notifyLog, setNotifyLog] = useState("");
  const [previews, setPreviews] = useState<Record<string, string>>({});
  const [detectingSources, setDetectingSources] = useState(false);
  const [detectNote, setDetectNote] = useState("");
  const [detectingAlist, setDetectingAlist] = useState(false);
  const [alistNote, setAlistNote] = useState("");
  const [pickingDir, setPickingDir] = useState(false);
  const [checkingDir, setCheckingDir] = useState(false);
  const [dirCheck, setDirCheck] = useState<LocalDirCheck | null>(null);
  const [authorizedDirs, setAuthorizedDirs] = useState<string[]>([]);
  const [loadingAuthorized, setLoadingAuthorized] = useState(false);
  const [dirPickerOpen, setDirPickerOpen] = useState(false);
  const autoDetect = useRef(false);

  useEffect(() => {
    const entries = Object.entries(config.magic_regex);
    if (entries.length === 0) {
      setPreviews({});
      return;
    }
    let cancelled = false;
    const timer = window.setTimeout(() => {
      Promise.all(
        entries.map(async ([key, rule]) => {
          try {
            const result = await previewRename({
              taskname: "示例任务",
              pattern: rule.pattern,
              replace: rule.replace,
              files: [sample],
            });
            const file = result.files[0];
            return [key, file?.matched ? file.renamed : "不匹配"] as const;
          } catch {
            return [key, "预览失败"] as const;
          }
        }),
      ).then((pairs) => {
        if (!cancelled) setPreviews(Object.fromEntries(pairs));
      });
    }, 300);
    return () => {
      cancelled = true;
      window.clearTimeout(timer);
    };
  }, [sample, config.magic_regex]);

  const update = <K extends keyof AppConfig>(key: K, value: AppConfig[K]) =>
    setConfig((prev) => ({ ...prev, [key]: value }));

  const updatePlugin = (
    plugin: string,
    key: string,
    value: string | number | boolean,
  ) =>
    setConfig((prev) => ({
      ...prev,
      plugins: {
        ...prev.plugins,
        [plugin]: { ...prev.plugins[plugin], [key]: value },
      },
    }));

  const updateSource = (
    source: "cloudsaver" | "pansou",
    key: string,
    value: string | boolean,
  ) =>
    setConfig((prev) => ({
      ...prev,
      source: {
        ...prev.source,
        [source]: { ...prev.source[source], [key]: value },
      },
    }));

  const updateLocalDownload = (
    key: keyof AppConfig["local_download"],
    value: string | number | boolean,
  ) =>
    setConfig((prev) => ({
      ...prev,
      local_download: { ...prev.local_download, [key]: value },
    }));

  const loadAuthorizedDirs = async () => {
    setLoadingAuthorized(true);
    try {
      const result = await getAuthorizedDirs();
      const paths = result.paths ?? [];
      setAuthorizedDirs(paths);
      if (paths.length > 0 && !config.local_download.dir.trim()) {
        updateLocalDownload("dir", paths[0]);
        void checkDirectory(paths[0]);
      }
    } catch {
      setAuthorizedDirs([]);
    } finally {
      setLoadingAuthorized(false);
    }
  };

  const checkDirectory = async (path: string) => {
    if (!path.trim()) {
      setDirCheck(null);
      return;
    }
    setCheckingDir(true);
    try {
      setDirCheck(await validateLocalDir(path.trim()));
    } catch (cause) {
      setDirCheck(null);
      onNotify(cause instanceof Error ? cause.message : "目录检查失败", "warn");
    } finally {
      setCheckingDir(false);
    }
  };

  const pickDirectory = async () => {
    setPickingDir(true);
    try {
      const dir = await pickSharedDirectory();
      if (!dir) return;
      updateLocalDownload("dir", dir);
      await checkDirectory(dir);
      void loadAuthorizedDirs();
    } catch (cause) {
      onNotify(
        cause instanceof Error ? cause.message : "无法打开飞牛目录选择器",
        "warn",
      );
    } finally {
      setPickingDir(false);
    }
  };

  const fillSearchSources = (result: SourceDetection) => {
    const found: string[] = [];
    let filled = 0;
    if (result.cloudsaver.online && result.cloudsaver.server) {
      found.push(
        `CloudSaver（${result.cloudsaver.server.replace(/^https?:\/\//, "")}）`,
      );
      if (!config.source.cloudsaver.server.trim()) {
        updateSource("cloudsaver", "server", result.cloudsaver.server);
        updateSource("cloudsaver", "enable", true);
        filled += 1;
      }
    }
    if (result.pansou.online && result.pansou.server) {
      found.push(`PanSou（${result.pansou.server.replace(/^https?:\/\//, "")}）`);
      if (!config.source.pansou.server.trim()) {
        updateSource("pansou", "server", result.pansou.server);
        updateSource("pansou", "enable", true);
        filled += 1;
      }
    }
    if (found.length === 0) {
      setDetectNote(
        "未在本机 8008 / 50080 端口发现 CloudSaver 或 PanSou，请手动填写服务器地址。",
      );
    } else if (filled > 0) {
      setDetectNote(`已识别本机 ${found.join("、")}，已填入并启用，保存后生效。`);
    } else {
      setDetectNote(`已识别本机 ${found.join("、")}（已有配置，未覆盖）。`);
    }
  };

  const fillAlistServers = (result: SourceDetection) => {
    const server = result.alist.online ? result.alist.server ?? "" : "";
    if (!server) {
      setAlistNote(
        "未在本机 5244 / 9255 端口发现 Alist 或 OpenList，请手动填写服务器地址。",
      );
      return;
    }
    const host = server.replace(/^https?:\/\//, "");
    const targets = (["alist", "alist_strm_gen", "alist_sync"] as const).filter(
      (plugin) => !String(config.plugins[plugin]?.url ?? "").trim(),
    );
    if (targets.length === 0) {
      setAlistNote(`已识别 ${host}（已有配置，未覆盖）。`);
      return;
    }
    setConfig((prev) => {
      const plugins = { ...prev.plugins };
      for (const plugin of targets) {
        plugins[plugin] = { ...(plugins[plugin] ?? {}), url: server };
      }
      return { ...prev, plugins };
    });
    setAlistNote(
      `已识别本机 ${host}，已填入 ${targets
        .map((plugin) => pluginLabels[plugin] ?? plugin)
        .join("、")}，保存后生效。`,
    );
  };

  const runDetection = async (scope: "search" | "alist" | "all") => {
    if (scope !== "alist") setDetectingSources(true);
    if (scope !== "search") setDetectingAlist(true);
    try {
      const result = await detectSources();
      if (scope !== "alist") fillSearchSources(result);
      if (scope !== "search") fillAlistServers(result);
    } catch (cause) {
      const message = cause instanceof Error ? cause.message : "自动识别失败";
      if (scope !== "alist") setDetectNote(message);
      if (scope !== "search") setAlistNote(message);
    } finally {
      if (scope !== "alist") setDetectingSources(false);
      if (scope !== "search") setDetectingAlist(false);
    }
  };

  useEffect(() => {
    void loadAuthorizedDirs();
  }, []);

  useEffect(() => {
    if (autoDetect.current) return;
    autoDetect.current = true;
    const searchEmpty =
      !config.source.cloudsaver.server.trim() &&
      !config.source.pansou.server.trim();
    const alistEmpty = ["alist", "alist_strm_gen", "alist_sync"].every(
      (plugin) => !String(config.plugins[plugin]?.url ?? "").trim(),
    );
    if (searchEmpty || alistEmpty) {
      void runDetection(searchEmpty ? (alistEmpty ? "all" : "search") : "alist");
    }
  }, []);

  const updateMagic = (key: string, field: keyof MagicRegex, value: string) =>
    setConfig((prev) => ({
      ...prev,
      magic_regex: {
        ...prev.magic_regex,
        [key]: { ...prev.magic_regex[key], [field]: value },
      },
    }));

  const go = (id: SettingsSection) => {
    setSection(id);
    document.getElementById(`settings-${id}`)?.scrollIntoView({
      behavior: "smooth",
      block: "start",
    });
  };

  const handleTestNotify = async () => {
    setTestingNotify(true);
    try {
      const result = await testNotify();
      const failed = result.channels.filter((item) => !item.ok);
      const ok = result.channels.length - failed.length;
      onNotify(
        `推送测试：${ok}/${result.channels.length} 成功${
          failed.length > 0
            ? `，${failed.map((item) => item.channel).join("、")} 失败`
            : ""
        }`,
        failed.length > 0 ? "warn" : "ok",
      );
      setNotifyLog(result.log.trim());
    } catch (cause) {
      onNotify(cause instanceof Error ? cause.message : "推送测试失败", "warn");
    } finally {
      setTestingNotify(false);
    }
  };

  const pluginNames = Object.keys(config.plugins);
  const activePlugins = pluginNames.filter((plugin) => pluginStatus[plugin]).length;

  const save = async () => {
    setSaving(true);
    try {
      await onSave();
      onNotify("配置已保存，调度器已重载", "ok");
    } catch (cause) {
      onNotify(cause instanceof Error ? cause.message : "保存失败", "warn");
    } finally {
      setSaving(false);
    }
  };

  const testSearch = async () => {
    setTestingSearch(true);
    try {
      const result = await searchShares("测试");
      const suffix = result.messages.length
        ? `（${result.messages.join("；")}）`
        : "";
      onNotify(`搜索源返回 ${result.results.length} 条候选${suffix}`, "ok");
    } catch (cause) {
      onNotify(cause instanceof Error ? cause.message : "搜索失败", "warn");
    } finally {
      setTestingSearch(false);
    }
  };

  return (
    <div className="grid gap-4 lg:grid-cols-[172px_minmax(0,1fr)]">
      <aside className="lg:sticky lg:top-[76px] lg:self-start">
        <div className="grid grid-cols-3 gap-1 rounded-panel border border-line bg-panel p-1 lg:flex lg:flex-col">
          {sections.map((item) => {
            const Icon = item.icon;
            return (
              <button
                key={item.id}
                onClick={() => go(item.id)}
                className={cn(
                  "flex items-center gap-2 rounded-md px-2 py-1.5 text-left transition-colors lg:px-2.5 lg:py-2",
                  section === item.id
                    ? "bg-brand-soft text-brand-ink"
                    : "text-muted hover:bg-line-soft hover:text-ink",
                )}
              >
                <Icon className="h-3.5 w-3.5 shrink-0" strokeWidth={1.7} />
                <span className="min-w-0">
                  <span className="block text-[12px] font-medium">{item.label}</span>
                  <span className="hidden text-[12px] text-subtle lg:block">
                    {item.description}
                  </span>
                </span>
              </button>
            );
          })}
        </div>
      </aside>

      <div className="min-w-0 space-y-4">
        <div className="sticky top-14 z-10 -mx-1 flex flex-wrap items-center justify-between gap-3 border-b border-line/70 bg-canvas/95 px-1 py-2 backdrop-blur-sm lg:top-[60px]">
          <span className="min-w-0 flex-1" />
          <Button
            size="sm"
            className="shrink-0"
            disabled={saving}
            onClick={() => void save()}
          >
            {saving ? (
              <Loader2 className="h-3.5 w-3.5 animate-spin" />
            ) : (
              <Save className="h-3.5 w-3.5" />
            )}
            {saving ? "保存中" : "保存配置"}
          </Button>
        </div>

        <PageSection
          id="settings-cookie"
          title="Cookie"
          description="第一个账号执行转存，所有账号执行签到。建议使用手机验证码登录后抓取的完整 Cookie。"
          action={
            <div className="flex gap-2">
              <Button
                variant="secondary"
                size="sm"
                disabled={running}
                onClick={onSign}
              >
                <Zap className="h-3.5 w-3.5" />
                立即签到
              </Button>
              <Button
                variant="secondary"
                size="sm"
                onClick={() =>
                  update("cookie", [...config.cookie, ""])
                }
              >
                <Plus className="h-3.5 w-3.5" />
                添加账号
              </Button>
            </div>
          }
        >
          <div className="space-y-2.5">
            {config.cookie.map((value, index) => (
              <div key={index} className="rounded-md border border-line bg-line-soft/25 p-2.5">
                <div className="mb-1.5 flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <span className="grid h-5 w-5 place-items-center rounded-full bg-sidebar text-[12px] font-semibold text-white">
                      {index + 1}
                    </span>
                    <span className="text-[12px] font-medium text-ink">
                      {index === 0 ? "主账号 · 执行转存" : "签到账号"}
                    </span>
                    {index === 0 ? <Badge variant="info">转存</Badge> : <Badge>仅签到</Badge>}
                  </div>
                  <Button
                    variant="ghost"
                    size="sm"
                    className="text-danger hover:bg-danger-soft hover:text-danger"
                    onClick={() =>
                      update(
                        "cookie",
                        config.cookie.filter((_, itemIndex) => itemIndex !== index),
                      )
                    }
                  >
                    <Trash2 className="h-3.5 w-3.5" />
                    移除
                  </Button>
                </div>
                <Textarea
                  value={value}
                  onChange={(event) => {
                    const next = [...config.cookie];
                    next[index] = event.target.value;
                    update("cookie", next);
                  }}
                  placeholder="kps=...; sign=...; vcode=...; __uid=...; __pus=..."
                  className="h-16 font-mono text-[12px]"
                  aria-label={`第 ${index + 1} 个账号 Cookie`}
                />
              </div>
            ))}
            <div className="flex items-center gap-1.5 text-[12px] text-muted">
              <KeyRound className="h-3 w-3" />
              移动端参数（kps / sign / vcode）用于签到；缺失时只执行转存。
            </div>
          </div>
        </PageSection>

        <PageSection
          id="settings-schedule"
          title="定时"
          description="由 Go 服务端的 cron 调度器执行；保存配置后自动重新加载，同一时间只跑一轮。"
        >
          <div className="grid gap-3 md:grid-cols-[1fr_1.2fr]">
            <Field label="Crontab" hint="例：0 8,18,20 * * *（每天 8 点、18 点、20 点）">
              <Input
                value={config.crontab}
                onChange={(event) => update("crontab", event.target.value)}
                className="font-mono"
              />
            </Field>
            <div className="rounded-md border border-line bg-line-soft/30 px-3 py-2">
              <div className="text-[12px] text-muted">下一次运行</div>
              <div className="mt-1.5 flex flex-wrap gap-1.5">
                {(config.next_runs.length > 0 ? config.next_runs : ["保存后计算"]).map(
                  (item) => (
                    <Badge key={item} variant="neutral">
                      <CalendarClock className="h-3 w-3" />
                      {item}
                    </Badge>
                  ),
                )}
              </div>
              <div className="mt-2 font-mono text-[12px] text-subtle">
                单实例运行 · 错过不补跑 · 保存后立即重载
              </div>
            </div>
          </div>
          <div className="mt-3 max-w-[220px]">
            <Field label="运行日志保留条数" hint="超出后自动删除最旧记录。">
              <Input
                type="number"
                min={1}
                max={200}
                value={config.run_logs.keep}
                onChange={(event) =>
                  update("run_logs", { keep: Number(event.target.value) })
                }
              />
            </Field>
          </div>
        </PageSection>

        <PageSection
          id="settings-notify"
          title="通知"
          description="任务结束、签到结果与失效提醒统一汇总；支持多渠道并行推送。"
          action={
            <Button
              variant="secondary"
              size="sm"
              disabled={
                testingNotify || Object.keys(config.push_config).length === 0
              }
              onClick={() => void handleTestNotify()}
            >
              {testingNotify ? (
                <Loader2 className="h-3.5 w-3.5 animate-spin" />
              ) : (
                <Bell className="h-3.5 w-3.5" />
              )}
              发送测试
            </Button>
          }
        >
          {Object.keys(config.push_config).length > 0 ? (
            <div className="overflow-hidden rounded-md border border-line">
              {Object.entries(config.push_config).map(([key, value]) => (
                <div
                  key={key}
                  className="flex items-center gap-3 border-b border-line-soft px-3 py-2 last:border-b-0"
                >
                  <span className="w-[132px] shrink-0 text-[12px] font-medium text-ink">
                    {pushLabels[key] ?? key}
                  </span>
                  <span className="font-mono text-[12px] uppercase tracking-[0.08em] text-subtle">
                    {key}
                  </span>
                  <Input
                    value={String(value)}
                    onChange={(event) =>
                      setConfig((prev) => ({
                        ...prev,
                        push_config: { ...prev.push_config, [key]: event.target.value },
                      }))
                    }
                    type={isSecret(key) ? "password" : "text"}
                    className="ml-auto h-7 max-w-[320px] font-mono text-[12px]"
                    aria-label={key}
                  />
                  <Button
                    variant="ghost"
                    size="icon-sm"
                    aria-label={`删除 ${key}`}
                    onClick={() =>
                      setConfig((prev) => {
                        const next = { ...prev.push_config };
                        delete next[key];
                        return { ...prev, push_config: next };
                      })
                    }
                  >
                    <Trash2 className="h-3.5 w-3.5" />
                  </Button>
                </div>
              ))}
            </div>
          ) : (
            <p className="text-[12px] text-muted">
              还没有添加通知渠道；从下面选择一项后填写参数。
            </p>
          )}
          <div className="mt-2 flex gap-2">
            <Select
              value={newChannel || "__none__"}
              onValueChange={(value) =>
                setNewChannel(value === "__none__" ? "" : value)
              }
            >
              <SelectTrigger className="max-w-[240px]" aria-label="选择要添加的通知渠道">
                <SelectValue placeholder="添加通知渠道…" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="__none__">添加通知渠道…</SelectItem>
                {Object.keys(pushLabels)
                  .filter((key) => !(key in config.push_config))
                  .map((key) => (
                    <SelectItem key={key} value={key}>
                      {pushLabels[key]}
                    </SelectItem>
                  ))}
              </SelectContent>
            </Select>
            <Button
              variant="secondary"
              size="sm"
              disabled={!newChannel}
              onClick={() => {
                if (!newChannel) return;
                setConfig((prev) => ({
                  ...prev,
                  push_config: { ...prev.push_config, [newChannel]: "" },
                }));
                setNewChannel("");
                onNotify("已添加通知渠道，填写后保存", "ok");
              }}
            >
              <Plus className="h-3.5 w-3.5" />
              添加
            </Button>
          </div>
          {notifyLog ? (
            <pre className="mt-2 max-h-48 overflow-auto whitespace-pre-wrap rounded-md border border-line bg-line-soft/40 p-2.5 font-mono text-[12px] leading-relaxed text-muted">
              {notifyLog}
            </pre>
          ) : null}
        </PageSection>

        <PageSection
          id="settings-download"
          title="下载"
          description="把云端文件直接落盘到 NAS；目录须位于存储空间下，且应用用户可写。"
        >
          <div className="space-y-3">
            <div className="rounded-md border border-line px-3 py-2.5">
              {loadingAuthorized || authorizedDirs.length > 0 ? (
                <div className="mb-2 flex flex-wrap items-center gap-1.5 text-[12px] text-muted">
                  <span>飞牛已授权</span>
                  {loadingAuthorized ? (
                    <Loader2 className="h-3 w-3 animate-spin" />
                  ) : null}
                  {authorizedDirs.map((path) => (
                    <button
                      key={path}
                      type="button"
                      onClick={() => {
                        updateLocalDownload("dir", path);
                        void checkDirectory(path);
                      }}
                      className={cn(
                        "max-w-full truncate rounded border px-1.5 py-0.5 font-mono text-[11px] transition-colors",
                        config.local_download.dir === path
                          ? "border-brand-ink/30 bg-brand-soft text-brand-ink"
                          : "border-line text-subtle hover:bg-line-soft hover:text-ink",
                      )}
                    >
                      {path}
                    </button>
                  ))}
                </div>
              ) : null}
              <Field
                label="默认下载目录"
              >
                <div className="flex flex-wrap gap-2">
                  <Input
                    value={config.local_download.dir}
                    onChange={(event) =>
                      updateLocalDownload("dir", event.target.value)
                    }
                    onBlur={() =>
                      void checkDirectory(config.local_download.dir)
                    }
                    className="min-w-0 flex-1 font-mono text-[12px]"
                    placeholder="/vol1/1000/Media"
                    aria-label="默认下载目录"
                  />
                  <Button
                    variant="secondary"
                    size="sm"
                    onClick={() => setDirPickerOpen(true)}
                  >
                    <FolderOpen className="h-3.5 w-3.5" />
                    选择目录
                  </Button>
                  <Button
                    variant="ghost"
                    size="sm"
                    disabled={pickingDir}
                    onClick={() => void pickDirectory()}
                  >
                    {pickingDir ? (
                      <Loader2 className="h-3.5 w-3.5 animate-spin" />
                    ) : (
                      <Plus className="h-3.5 w-3.5" />
                    )}
                    新增授权
                  </Button>
                  <Button
                    variant="ghost"
                    size="sm"
                    disabled={checkingDir || !config.local_download.dir.trim()}
                    onClick={() => void checkDirectory(config.local_download.dir)}
                  >
                    {checkingDir ? (
                      <Loader2 className="h-3.5 w-3.5 animate-spin" />
                    ) : (
                      <HardDrive className="h-3.5 w-3.5" />
                    )}
                    检查
                  </Button>
                </div>
              </Field>
              {dirCheck ? (
                <div
                  className={cn(
                    "mt-2 flex items-center gap-1.5 rounded border px-2.5 py-1.5 text-[12px]",
                    dirCheck.ok
                      ? "border-ok/30 bg-ok-soft/40 text-ok"
                      : "border-danger/30 bg-danger-soft/40 text-danger",
                  )}
                >
                  {dirCheck.ok ? (
                    <CheckCircle2 className="h-3.5 w-3.5" />
                  ) : (
                    <AlertTriangle className="h-3.5 w-3.5" />
                  )}
                  {dirCheck.ok
                    ? `目录可写，剩余 ${formatFree(dirCheck.free_bytes)}`
                    : dirCheck.message || "目录不可写"}
                </div>
              ) : null}
            </div>
            <div className="grid gap-3 md:grid-cols-3">
              <Field label="并发下载数">
                <Input
                  type="number"
                  min={1}
                  max={8}
                  value={config.local_download.concurrency}
                  onChange={(event) =>
                    updateLocalDownload("concurrency", Number(event.target.value))
                  }
                />
              </Field>
              <Field label="最小可用空间 (GB)" hint="低于该值时跳过下载">
                <Input
                  type="number"
                  min={0}
                  value={config.local_download.min_free_gb}
                  onChange={(event) =>
                    updateLocalDownload("min_free_gb", Number(event.target.value))
                  }
                />
              </Field>
              <div className="rounded-md border border-line px-3 py-2.5">
                <SwitchRow
                  label="覆盖同名文件"
                  description="关闭时保留已有文件"
                  checked={config.local_download.overwrite}
                  onCheckedChange={(checked) =>
                    updateLocalDownload("overwrite", checked)
                  }
                />
              </div>
            </div>
          </div>
        </PageSection>

        <PageSection
          id="settings-plugins"
          title="插件"
          description="按固定顺序在转存后运行；PLUGIN_FLAGS 环境变量可在启动时排除插件。"
          action={
            <div className="flex items-center gap-2">
              <Badge variant="ok" dot>
                已连接 {activePlugins} / {pluginNames.length}
              </Badge>
              <Button
                variant="ghost"
                size="sm"
                disabled={detectingAlist}
                onClick={() => void runDetection("alist")}
              >
                {detectingAlist ? (
                  <Loader2 className="h-3.5 w-3.5 animate-spin" />
                ) : (
                  <Radar className="h-3.5 w-3.5" />
                )}
                自动识别 Alist
              </Button>
            </div>
          }
          bodyClassName="p-0"
        >
          {alistNote ? (
            <div className="border-b border-line bg-line-soft/40 px-4 py-2 text-[12px] text-muted">
              {alistNote}
            </div>
          ) : null}
          <div className="divide-y divide-line-soft">
            {pluginNames.map((plugin) => {
              const open = openPlugin === plugin;
              const values = config.plugins[plugin] ?? {};
              return (
                <div key={plugin}>
                  <button
                    className="flex w-full items-center gap-3 px-4 py-3 text-left transition-colors hover:bg-line-soft/30"
                    onClick={() => setOpenPlugin(open ? null : plugin)}
                    aria-expanded={open}
                  >
                    <span
                      className={cn(
                        "grid h-7 w-7 shrink-0 place-items-center rounded-md",
                        pluginStatus[plugin]
                          ? "bg-ok-soft text-ok"
                          : "bg-line-soft text-muted",
                      )}
                    >
                      <Plug className="h-3.5 w-3.5" />
                    </span>
                    <span className="min-w-0 flex-1">
                      <span className="flex items-center gap-1.5">
                        <span className="truncate text-[13px] font-medium text-ink">
                          {pluginLabels[plugin] ?? plugin}
                        </span>
                      </span>
                      <span className="mt-0.5 block truncate text-[12px] text-muted">
                        {pluginDescriptions[plugin] ?? ""}
                      </span>
                    </span>
                    <Badge variant={pluginStatus[plugin] ? "ok" : "neutral"} dot>
                      {pluginStatus[plugin] ? "已连接" : "未配置"}
                    </Badge>
                    <ChevronDown
                      className={cn(
                        "h-3.5 w-3.5 shrink-0 text-subtle transition-transform",
                        open && "rotate-180",
                      )}
                    />
                  </button>
                  {open ? (
                    <div className="border-t border-line-soft bg-line-soft/20 px-4 py-3">
                      <PluginFields
                        plugin={plugin}
                        values={values}
                        onChange={(key, value) => updatePlugin(plugin, key, value)}
                      />
                      <div className="mt-2 flex items-center justify-between border-t border-line-soft pt-2">
                        <span className="font-mono text-[12px] uppercase tracking-[0.08em] text-subtle">
                          {plugin} · default_config
                        </span>
                      </div>
                    </div>
                  ) : null}
                </div>
              );
            })}
          </div>
        </PageSection>

        <PageSection
          id="settings-magic"
          title="魔法正则"
          description="匹配表达式可写 $TV_MAGIC 等魔法键；替换式支持 {TASKNAME}、{SXX}、{E}、{EXT} 等变量。"
          action={
            <Button
              variant="secondary"
              size="sm"
              onClick={() => {
                const key = `$MAGIC_${Object.keys(config.magic_regex).length + 1}`;
                update("magic_regex", {
                  ...config.magic_regex,
                  [key]: { pattern: "", replace: "" },
                });
                onNotify(`已添加 ${key}`, "ok");
              }}
            >
              <Plus className="h-3.5 w-3.5" />
              添加规则
            </Button>
          }
        >
          <div className="mb-3 flex flex-wrap items-center gap-2 rounded-md border border-line bg-line-soft/30 px-3 py-2">
            <span className="text-[12px] text-muted">样本文件名</span>
            <Input
              value={sample}
              onChange={(event) => setSample(event.target.value)}
              className="h-7 max-w-[280px] font-mono text-[12px]"
              aria-label="样本文件名"
            />
            <span className="font-mono text-[12px] text-subtle">
              用于下方每条规则的即时预览
            </span>
          </div>
          <div className="space-y-2">
            {Object.entries(config.magic_regex).map(([key, value]) => {
              const preview = previews[key] ?? "预览中…";
              return (
                <div key={key} className="rounded-md border border-line px-3 py-2.5">
                  <div className="flex items-center gap-2">
                    <span className="font-mono text-[12px] font-medium text-brand-ink">
                      {key}
                    </span>
                    <span className="ml-auto font-mono text-[12px] text-subtle">
                      {preview}
                    </span>
                    <Button
                      variant="ghost"
                      size="icon-sm"
                      aria-label={`删除 ${key}`}
                      onClick={() =>
                        setConfig((prev) => {
                          const next = { ...prev.magic_regex };
                          delete next[key];
                          return { ...prev, magic_regex: next };
                        })
                      }
                    >
                      <Trash2 className="h-3.5 w-3.5" />
                    </Button>
                  </div>
                  <div className="mt-1.5 grid gap-2 md:grid-cols-[1.4fr_1fr]">
                    <Field label="匹配表达式">
                      <Input
                        value={value.pattern}
                        onChange={(event) =>
                          updateMagic(key, "pattern", event.target.value)
                        }
                        className="font-mono text-[12px]"
                      />
                    </Field>
                    <Field label="替换表达式">
                      <Input
                        value={value.replace}
                        onChange={(event) =>
                          updateMagic(key, "replace", event.target.value)
                        }
                        className="font-mono text-[12px]"
                        placeholder="留空则使用魔法变量"
                      />
                    </Field>
                  </div>
                </div>
              );
            })}
          </div>
        </PageSection>

        <PageSection
          id="settings-source"
          title="搜索源"
          description="任务名称搜索会并行查询已启用的源，并按发布时间排序去重。"
          action={
            <div className="flex items-center gap-2">
              <Button
                variant="ghost"
                size="sm"
                disabled={detectingSources}
                onClick={() => void runDetection("search")}
              >
                {detectingSources ? (
                  <Loader2 className="h-3.5 w-3.5 animate-spin" />
                ) : (
                  <Radar className="h-3.5 w-3.5" />
                )}
                自动识别
              </Button>
              <Button
                variant="secondary"
                size="sm"
                disabled={testingSearch}
                onClick={() => void testSearch()}
              >
                {testingSearch ? (
                  <Loader2 className="h-3.5 w-3.5 animate-spin" />
                ) : (
                  <Search className="h-3.5 w-3.5" />
                )}
                测试搜索
              </Button>
            </div>
          }
        >
          <div className="space-y-4">
            {detectNote ? (
              <div className="rounded border border-line bg-line-soft/40 px-3 py-2 text-[12px] text-muted">
                {detectNote}
              </div>
            ) : null}
            <div className="rounded-md border border-line px-3 py-2.5">
              <SwitchRow
                label="CloudSaver"
                description="需要服务器地址与账号；token 失效时自动重新登录。"
                checked={config.source.cloudsaver.enable}
                onCheckedChange={(checked) =>
                  updateSource("cloudsaver", "enable", checked)
                }
              />
              <div className="grid gap-2 md:grid-cols-2">
                <Field label="服务器">
                  <Input
                    value={config.source.cloudsaver.server}
                    onChange={(event) =>
                      updateSource("cloudsaver", "server", event.target.value)
                    }
                    className="font-mono text-[12px]"
                  />
                </Field>
                <Field label="用户名">
                  <Input
                    value={config.source.cloudsaver.username}
                    onChange={(event) =>
                      updateSource("cloudsaver", "username", event.target.value)
                    }
                  />
                </Field>
                <Field label="密码">
                  <Input
                    type="password"
                    value={config.source.cloudsaver.password}
                    onChange={(event) =>
                      updateSource("cloudsaver", "password", event.target.value)
                    }
                  />
                </Field>
                <Field label="Token">
                  <Input
                    type="password"
                    value={config.source.cloudsaver.token}
                    onChange={(event) =>
                      updateSource("cloudsaver", "token", event.target.value)
                    }
                  />
                </Field>
              </div>
            </div>
            <div className="rounded-md border border-line px-3 py-2.5">
              <SwitchRow
                label="PanSou"
                description="只需服务器地址；按关键词返回夸克网盘分享。"
                checked={config.source.pansou.enable}
                onCheckedChange={(checked) =>
                  updateSource("pansou", "enable", checked)
                }
              />
              <Field label="服务器">
                <Input
                  value={config.source.pansou.server}
                  onChange={(event) =>
                    updateSource("pansou", "server", event.target.value)
                  }
                  className="font-mono text-[12px]"
                />
              </Field>
            </div>
          </div>
        </PageSection>

      </div>

      <LocalDirDialog
        open={dirPickerOpen}
        initial={config.local_download.dir}
        onOpenChange={setDirPickerOpen}
        onSelect={(path) => {
          updateLocalDownload("dir", path);
          void checkDirectory(path);
          onNotify(`下载目录已设为 ${path}`, "ok");
        }}
        onNotify={onNotify}
      />
    </div>
  );
}
