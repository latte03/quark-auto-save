import { useCallback, useEffect, useRef, useState } from "react";
import {
  AlertTriangle,
  Bell,
  Check,
  CheckCircle2,
  ChevronDown,
  CircleAlert,
  CircleHelp,
  Cookie,
  FileText,
  FolderOpen,
  HardDrive,
  KeyRound,
  Loader2,
  Pencil,
  Plug,
  Palette,
  Plus,
  QrCode,
  Radar,
  RefreshCw,
  Save,
  Search,
  Sparkles,
  Trash2,
  X,
  Zap,
} from "lucide-react";
import { QRCodeSVG } from "qrcode.react";
import type { AppConfig, PluginConfig } from "../lib/types";
import { pluginDescriptions, pluginLabels } from "../lib/mock";
import {
  detectSources,
  getAuthorizedDirs,
  getConfig,
  getQuarkAccount,
  pollQuarkQR,
  previewRename,
  renameMagicRule,
  searchShares,
  startQuarkQR,
  testNotify,
  validateLocalDir,
  type LocalDirCheck,
  type QuarkAccount,
  type QuarkQR,
  type SourceDetection,
} from "../lib/api";
import { pickSharedDirectory } from "../lib/trim";
import { LocalDirDialog } from "./LocalDirDialog";
import { cn } from "../lib/utils";
import { Badge } from "../components/ui/badge";
import { Button } from "../components/ui/button";
import {
  Field,
  FieldDescription,
  FieldLabel,
} from "../components/ui/field";
import { SwitchField } from "../components/SwitchField";
import { Switch } from "../components/ui/switch";
import { Input } from "../components/ui/input";
import { Textarea } from "../components/ui/textarea";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "../components/ui/select";
import {
  Dialog,
  DialogBody,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "../components/ui/dialog";
import {
  HoverCard,
  HoverCardContent,
  HoverCardTrigger,
} from "../components/ui/hover-card";
import { PageHeader } from "../components/PageHeader";
import { PageSection } from "../components/PageSection";
import { Tabs, TabsList, TabsTrigger } from "../components/ui/tabs";
import type { ToastTone } from "../components/Toast";
import { themeLabels, type ThemeMode } from "../lib/theme";

type SettingsSection =
  | "appearance"
  | "cookie"
  | "logs"
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
  { id: "appearance", label: "外观", icon: Palette, description: "浅色与深色" },
  { id: "cookie", label: "Cookie", icon: Cookie, description: "账号与签到" },
  { id: "logs", label: "日志", icon: FileText, description: "运行记录" },
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

function newRuleId() {
  if (typeof crypto !== "undefined" && "randomUUID" in crypto) {
    return `m${crypto.randomUUID().replaceAll("-", "").slice(0, 8)}`;
  }
  return `m${Math.random().toString(36).slice(2, 10)}`;
}

const magicVariables: { name: string; meaning: string }[] = [
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
              <SwitchField
                label={fieldLabel(key)}
                checked={value}
                onCheckedChange={(checked) => onChange(key, checked)}
              />
            </div>
          );
        }
        if (typeof value === "number") {
          return (
            <Field key={key} className="py-1.5">
              <FieldLabel>{fieldLabel(key)}</FieldLabel>
              <Input
                type="number"
                value={value}
                onChange={(event) => onChange(key, Number(event.target.value))}
              />
            </Field>
          );
        }
        return (
          <Field key={key} className="py-1.5">
            <FieldLabel>{fieldLabel(key)}</FieldLabel>
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
  dirty: boolean;
  theme: ThemeMode;
  onThemeChange: (mode: ThemeMode) => void;
  onAdoptDefaults: () => void;
  onSign: () => void;
  onSave: () => Promise<void>;
  onTasksReload: () => void;
  onNotify: (message: string, tone?: ToastTone) => void;
};

export function SettingsPage({
  config,
  setConfig,
  pluginStatus,
  running,
  dirty,
  theme,
  onThemeChange,
  onAdoptDefaults,
  onSign,
  onSave,
  onTasksReload,
  onNotify,
}: SettingsPageProps) {
  const [section, setSection] = useState<SettingsSection>("cookie");
  const [openPlugin, setOpenPlugin] = useState<string | null>(
    Object.keys(config.plugins)[0] ?? null,
  );
  const [sample, setSample] = useState("Black.Mirror.S07E05.mkv");
  const [renamingKey, setRenamingKey] = useState<string | null>(null);
  const [renameDraft, setRenameDraft] = useState("");
  const [renamingBusy, setRenamingBusy] = useState(false);
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
  const [authorizedError, setAuthorizedError] = useState("");
  const [loadingAuthorized, setLoadingAuthorized] = useState(false);
  const [dirPickerOpen, setDirPickerOpen] = useState(false);
  const [qrOpen, setQrOpen] = useState(false);
  const [qr, setQr] = useState<QuarkQR | null>(null);
  const [qrState, setQrState] = useState<"waiting" | "confirmed" | "expired" | "error">(
    "waiting",
  );
  const [qrNote, setQrNote] = useState("");
  const [qrNick, setQrNick] = useState("");
  const [startingQR, setStartingQR] = useState(false);
  const [account, setAccount] = useState<QuarkAccount | null>(null);
  const [checkingAccount, setCheckingAccount] = useState(false);
  const autoDetect = useRef(false);

  const checkAccount = useCallback(async () => {
    setCheckingAccount(true);
    try {
      setAccount(await getQuarkAccount());
    } catch (cause) {
      setAccount({
        ok: false,
        message: cause instanceof Error ? cause.message : "账号校验失败",
      });
    } finally {
      setCheckingAccount(false);
    }
  }, []);

  useEffect(() => {
    void checkAccount();
  }, [checkAccount]);

  const openQR = async () => {
    setQrOpen(true);
    setQr(null);
    setQrNote("");
    setQrNick("");
    setStartingQR(true);
    try {
      const session = await startQuarkQR();
      setQr(session);
      setQrState("waiting");
    } catch (cause) {
      setQrState("error");
      setQrNote(cause instanceof Error ? cause.message : "无法获取登录二维码");
    } finally {
      setStartingQR(false);
    }
  };

  useEffect(() => {
    if (!qrOpen || !qr || qrState !== "waiting") return;
    let active = true;
    const timer = window.setInterval(() => {
      pollQuarkQR(qr.id)
        .then((next) => {
          if (!active) return;
          if (next.status === "confirmed") {
            setQrState("confirmed");
            setQrNick(next.nickname ?? "");
            setQrNote(next.nickname ? `登录成功：${next.nickname}` : "登录成功");
            if (next.cookies) {
              setConfig((prev) => ({ ...prev, cookie: next.cookies ?? prev.cookie }));
            }
            setAccount({
              ok: true,
              nickname: next.nickname,
              checked_at: new Date().toISOString(),
            });
            onNotify(
              `扫码登录成功${next.nickname ? `：${next.nickname}` : ""}`,
              "ok",
            );
          } else if (next.status === "expired") {
            setQrState("expired");
            setQrNote("二维码已过期，请刷新");
          } else if (next.status === "error") {
            setQrState("error");
            setQrNote(next.message || "扫码失败，请重试");
          }
        })
        .catch((cause: unknown) => {
          if (active) {
            setQrNote(cause instanceof Error ? cause.message : "查询扫码状态失败");
          }
        });
    }, 1200);
    return () => {
      active = false;
      window.clearInterval(timer);
    };
  }, [qrOpen, qr, qrState, setConfig, onNotify]);

  useEffect(() => {
    const rules = config.magic_regex;
    if (rules.length === 0) {
      setPreviews({});
      return;
    }
    let cancelled = false;
    const timer = window.setTimeout(() => {
      Promise.all(
        rules.map(async (rule) => {
          try {
            const result = await previewRename({
              taskname: "示例任务",
              magic_id: rule.id,
              pattern: rule.pattern,
              replace: rule.replace,
              files: [sample],
            });
            const file = result.files[0];
            return [rule.id, file?.matched ? file.renamed : "不匹配"] as const;
          } catch {
            return [rule.id, "预览失败"] as const;
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
    setAuthorizedError("");
    try {
      const result = await getAuthorizedDirs();
      const paths = result.paths ?? [];
      setAuthorizedDirs(paths);
      if (paths.length > 0 && !config.local_download.dir.trim()) {
        updateLocalDownload("dir", paths[0]);
        void checkDirectory(paths[0]);
        onAdoptDefaults();
      }
    } catch (cause) {
      setAuthorizedDirs([]);
      setAuthorizedError(
        cause instanceof Error ? cause.message : "无法读取飞牛授权目录",
      );
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
      onNotify(cause instanceof Error ? cause.message : "目录检查失败", "error");
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

  const runDetection = async (
    scope: "search" | "alist" | "all",
    adopt = false,
  ) => {
    if (scope !== "alist") setDetectingSources(true);
    if (scope !== "search") setDetectingAlist(true);
    try {
      const result = await detectSources();
      if (scope !== "alist") fillSearchSources(result);
      if (scope !== "search") fillAlistServers(result);
      if (adopt) onAdoptDefaults();
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
      void runDetection(
        searchEmpty ? (alistEmpty ? "all" : "search") : "alist",
        true,
      );
    }
  }, []);

  const updateMagic = (
    id: string,
    field: "name" | "pattern" | "replace",
    value: string,
  ) =>
    setConfig((prev) => ({
      ...prev,
      magic_regex: prev.magic_regex.map((rule) =>
        rule.id === id ? { ...rule, [field]: value } : rule,
      ),
    }));

  // Renaming only touches the display name; tasks reference the stable id.
  const commitRename = async (id: string) => {
    const current = config.magic_regex.find((rule) => rule.id === id);
    const to = renameDraft.trim();
    if (!current || !to || to === current.name) {
      setRenamingKey(null);
      return;
    }
    if (config.magic_regex.some((rule) => rule.id !== id && rule.name === to)) {
      onNotify(`已存在规则 ${to}`, "warn");
      return;
    }
    setRenamingBusy(true);
    try {
      await renameMagicRule(id, to);
      setConfig(await getConfig());
      setRenamingKey(null);
      onNotify(`已重命名为 ${to}`, "ok");
      onTasksReload();
    } catch (cause) {
      onNotify(cause instanceof Error ? cause.message : "重命名失败", "error");
    } finally {
      setRenamingBusy(false);
    }
  };

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
      onNotify(cause instanceof Error ? cause.message : "推送测试失败", "error");
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
      onNotify(cause instanceof Error ? cause.message : "保存失败", "error");
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
      onNotify(cause instanceof Error ? cause.message : "搜索失败", "error");
    } finally {
      setTestingSearch(false);
    }
  };

  return (
    <div className="space-y-4">
      <PageHeader
        title="设置"
        description="账号、通知、插件与命名规则。"
      />
      <div className="grid gap-4 lg:grid-cols-[172px_minmax(0,1fr)]">
      <aside className="lg:sticky lg:top-5 lg:self-start">
        <div className="grid grid-cols-3 gap-1 lg:flex lg:flex-col lg:gap-0.5">
          {sections.map((item) => {
            const Icon = item.icon;
            const active = section === item.id;
            return (
              <button
                key={item.id}
                onClick={() => go(item.id)}
                aria-current={active ? "true" : undefined}
                className={cn(
                  "relative flex items-center gap-2 rounded-md px-2 py-1.5 text-left transition-colors lg:px-2.5 lg:py-2",
                  active
                    ? "bg-brand text-white"
                    : "text-muted-ink hover:bg-line-soft hover:text-ink",
                )}
              >
                <Icon className="h-3.5 w-3.5 shrink-0" strokeWidth={1.7} />
                <span className="min-w-0">
                  <span
                    className={cn(
                      "block text-[12px]",
                      active ? "font-semibold" : "font-medium",
                    )}
                  >
                    {item.label}
                  </span>
                  <span
                    className={cn(
                      "hidden text-[12px] lg:block",
                      active ? "text-white/70" : "text-subtle",
                    )}
                  >
                    {item.description}
                  </span>
                </span>
              </button>
            );
          })}
        </div>
      </aside>

      <div className="min-w-0 space-y-4">
        <PageSection
          id="settings-appearance"
          title="外观"
          description="只影响当前浏览器，不写入配置文件。"
        >
          <Tabs
            value={theme}
            onValueChange={(value) => onThemeChange(value as ThemeMode)}
            className="max-w-[320px]"
          >
            <TabsList className="w-full">
              {(Object.keys(themeLabels) as ThemeMode[]).map((mode) => (
                <TabsTrigger key={mode} value={mode} className="text-[12px]">
                  {themeLabels[mode]}
                </TabsTrigger>
              ))}
            </TabsList>
          </Tabs>
        </PageSection>

        <PageSection
          id="settings-cookie"
          title="Cookie"
          description="第一个账号执行转存，所有账号执行签到。可以扫码登录直接写入主账号，也可以手动粘贴 Cookie。"
          action={
            <div className="flex gap-2">
              <Button variant="secondary" size="sm" onClick={() => void openQR()}>
                <QrCode className="h-3.5 w-3.5" />
                扫码登录
              </Button>
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
            {config.cookie.length > 0 ? (
              <div className="flex flex-wrap items-center gap-2 rounded-md bg-line-soft/60 px-3 py-2.5 text-[12px]">
                <span className="text-muted-ink">当前账号</span>
                {account?.ok ? (
                  <Badge variant="ok" dot>
                    {account.nickname || "已登录"}
                  </Badge>
                ) : account ? (
                  <Badge variant="danger" dot>
                    Cookie 失效
                  </Badge>
                ) : (
                  <Badge variant="neutral">未校验</Badge>
                )}
                {account?.ok && account.checked_at ? (
                  <span className="text-subtle">
                    校验于{" "}
                    {new Date(account.checked_at).toLocaleTimeString("zh-CN", {
                      hour12: false,
                    })}
                  </span>
                ) : null}
                {account && !account.ok && account.message ? (
                  <span className="min-w-0 truncate text-danger">{account.message}</span>
                ) : null}
                <Button
                  variant="ghost"
                  size="sm"
                  className="ml-auto"
                  disabled={checkingAccount}
                  onClick={() => void checkAccount()}
                >
                  {checkingAccount ? (
                    <Loader2 className="h-3.5 w-3.5 animate-spin" />
                  ) : (
                    <RefreshCw className="h-3.5 w-3.5" />
                  )}
                  重新校验
                </Button>
              </div>
            ) : null}
            {config.cookie.map((value, index) => (
              <div key={index}>
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
            <div className="flex items-center gap-1.5 text-[12px] text-muted-ink">
              <KeyRound className="h-3 w-3" />
              移动端参数（kps / sign / vcode）用于签到；缺失时只执行转存。
            </div>
          </div>
        </PageSection>

        <PageSection
          id="settings-logs"
          title="日志"
          description="任务与签到的输出会保存在 NAS 上，运行记录页按时间倒序展示。"
        >
          <Field className="max-w-[240px]">
            <FieldLabel htmlFor="run-log-keep">运行日志保留条数</FieldLabel>
            <Input
              id="run-log-keep"
              type="number"
              min={1}
              max={200}
              value={config.run_logs.keep}
              onChange={(event) =>
                update("run_logs", { keep: Number(event.target.value) })
              }
            />
            <FieldDescription>超出后自动删除最旧记录。</FieldDescription>
          </Field>
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
            <div className="space-y-1 rounded-md bg-line-soft/50 p-2">
              {Object.entries(config.push_config).map(([key, value]) => (
                <div
                  key={key}
                  className="flex items-center gap-3 px-1.5 py-1.5"
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
            <p className="text-[12px] text-muted-ink">
              还没有添加通知渠道；从下面选择一项后填写参数。
            </p>
          )}
          <div className="mt-2 flex gap-2">
            <Select
              value={newChannel || "__none__"}
              items={[
                { value: "__none__", label: "添加通知渠道…" },
                ...Object.keys(pushLabels)
                  .filter((key) => !(key in config.push_config))
                  .map((key) => ({ value: key, label: pushLabels[key] ?? key })),
              ]}
              onValueChange={(value) =>
                setNewChannel(!value || value === "__none__" ? "" : value)
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
            <pre className="mt-2 max-h-48 overflow-auto whitespace-pre-wrap rounded-md bg-line-soft/50 p-2.5 font-mono text-[12px] leading-relaxed text-muted-ink">
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
            <div>
              {authorizedError ? (
                <div
                  role="alert"
                  className="mb-2 flex flex-wrap items-center gap-2 rounded-md bg-danger-soft/60 px-2.5 py-2 text-[12px] text-danger"
                >
                  <CircleAlert className="h-3.5 w-3.5 shrink-0" />
                  <span className="min-w-0 flex-1">{authorizedError}</span>
                  <Button
                    variant="ghost"
                    size="sm"
                    className="h-6 px-1.5 text-[12px] hover:bg-danger-soft hover:text-danger"
                    onClick={() => void loadAuthorizedDirs()}
                  >
                    重试
                  </Button>
                </div>
              ) : null}
              {loadingAuthorized || authorizedDirs.length > 0 ? (
                <div className="mb-2 flex flex-wrap items-center gap-1.5 text-[12px] text-muted-ink">
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
              <Field>
                <FieldLabel htmlFor="download-dir">默认下载目录</FieldLabel>
                <div className="flex flex-wrap items-center gap-2">
                  <Input
                    id="download-dir"
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
              <Field>
                <FieldLabel htmlFor="download-concurrency">并发下载数</FieldLabel>
                <Input
                  id="download-concurrency"
                  type="number"
                  min={1}
                  max={8}
                  value={config.local_download.concurrency}
                  onChange={(event) =>
                    updateLocalDownload("concurrency", Number(event.target.value))
                  }
                />
              </Field>
              <Field>
                <FieldLabel htmlFor="download-min-free">最小可用空间 (GB)</FieldLabel>
                <Input
                  id="download-min-free"
                  type="number"
                  min={0}
                  value={config.local_download.min_free_gb}
                  onChange={(event) =>
                    updateLocalDownload("min_free_gb", Number(event.target.value))
                  }
                />
                <FieldDescription>低于该值时跳过下载</FieldDescription>
              </Field>
              <Field>
                <FieldLabel htmlFor="download-overwrite">覆盖同名文件</FieldLabel>
                <div className="flex h-8 items-center gap-2">
                  <Switch
                    id="download-overwrite"
                    checked={config.local_download.overwrite}
                    onCheckedChange={(checked) =>
                      updateLocalDownload("overwrite", checked)
                    }
                  />
                  <span className="text-[12px] text-muted-ink">
                    关闭时保留已有文件
                  </span>
                </div>
              </Field>
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
            <div className="bg-line-soft/50 px-4 py-2 text-[12px] text-muted-ink">
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
                          : "bg-line-soft text-muted-ink",
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
                      <span className="mt-0.5 block truncate text-[12px] text-muted-ink">
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
                    <div className="bg-line-soft/30 px-4 py-3">
                      <PluginFields
                        plugin={plugin}
                        values={values}
                        onChange={(key, value) => updatePlugin(plugin, key, value)}
                      />
                      <div className="mt-2 flex items-center justify-between">
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
          description={
            <span className="inline-flex flex-wrap items-center gap-x-1 gap-y-0.5">
              匹配表达式可写 $TV_MAGIC 等魔法键；替换表达式支持变量。
              <HoverCard>
                <HoverCardTrigger
                  render={
                    <button
                      type="button"
                      className="inline-flex items-center gap-0.5 rounded text-brand-ink outline-none hover:underline focus-visible:ring-2 focus-visible:ring-ring/50"
                    />
                  }
                >
                  <CircleHelp className="h-3 w-3" />
                  变量说明
                </HoverCardTrigger>
                <HoverCardContent align="start" className="w-[330px] p-3">
                  <div className="text-[12px] font-medium text-ink">
                    替换表达式可用变量
                  </div>
                  <dl className="mt-1.5 grid grid-cols-[auto_1fr] gap-x-3 gap-y-1">
                    {magicVariables.map((item) => (
                      <div key={item.name} className="contents">
                        <dt className="font-mono text-[12px] text-brand-ink">
                          {item.name}
                        </dt>
                        <dd className="text-[12px] leading-4 text-muted-ink">
                          {item.meaning}
                        </dd>
                      </div>
                    ))}
                  </dl>
                  <p className="mt-2 text-[12px] leading-4 text-subtle">
                    变量从原文件名抓取，抓不到时替换为空串。
                  </p>
                </HoverCardContent>
              </HoverCard>
            </span>
          }
          action={
            <Button
              variant="secondary"
              size="sm"
              onClick={() => {
                const name = `$MAGIC_${config.magic_regex.length + 1}`;
                update("magic_regex", [
                  ...config.magic_regex,
                  { id: newRuleId(), name, pattern: "", replace: "" },
                ]);
                onNotify(`已添加 ${name}`, "ok");
              }}
            >
              <Plus className="h-3.5 w-3.5" />
              添加规则
            </Button>
          }
        >
          <div className="mb-3 flex flex-wrap items-center gap-2">
            <span className="text-[12px] text-muted-ink">样本文件名</span>
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
            {config.magic_regex.map((rule) => {
              const key = rule.name;
              const preview = previews[rule.id] ?? "预览中…";
              return (
                <div key={rule.id} className="rounded-md bg-line-soft/50 px-3 py-2.5">
                  <div className="group flex items-center gap-2">
                    {renamingKey === rule.id ? (
                      <>
                        <Input
                          autoFocus
                          value={renameDraft}
                          aria-label={`规则名称 ${key}`}
                          className="h-7 w-48 font-mono text-[12px]"
                          onChange={(event) => setRenameDraft(event.target.value)}
                          onKeyDown={(event) => {
                            if (event.key === "Enter") void commitRename(rule.id);
                            if (event.key === "Escape") setRenamingKey(null);
                          }}
                        />
                        <Button
                          variant="ghost"
                          size="icon-sm"
                          aria-label="保存规则名称"
                          disabled={renamingBusy}
                          onClick={() => void commitRename(rule.id)}
                        >
                          {renamingBusy ? (
                            <Loader2 className="h-3.5 w-3.5 animate-spin" />
                          ) : (
                            <Check className="h-3.5 w-3.5" />
                          )}
                        </Button>
                        <Button
                          variant="ghost"
                          size="icon-sm"
                          aria-label="取消重命名"
                          onClick={() => setRenamingKey(null)}
                        >
                          <X className="h-3.5 w-3.5" />
                        </Button>
                      </>
                    ) : (
                      <>
                        <span className="font-mono text-[12px] font-medium text-brand-ink">
                          {key}
                        </span>
                        <Button
                          variant="ghost"
                          size="icon-sm"
                          aria-label={`重命名 ${key}`}
                          className="opacity-0 transition-opacity group-hover:opacity-100 focus-visible:opacity-100"
                          onClick={() => {
                            setRenamingKey(rule.id);
                            setRenameDraft(rule.name);
                          }}
                        >
                          <Pencil className="h-3.5 w-3.5" />
                        </Button>
                      </>
                    )}
                    <span className="ml-auto font-mono text-[12px] text-subtle">
                      {preview}
                    </span>
                    <Button
                      variant="ghost"
                      size="icon-sm"
                      aria-label={`删除 ${key}`}
                      onClick={() =>
                        setConfig((prev) => ({
                          ...prev,
                          magic_regex: prev.magic_regex.filter(
                            (item) => item.id !== rule.id,
                          ),
                        }))
                      }
                    >
                      <Trash2 className="h-3.5 w-3.5" />
                    </Button>
                  </div>
                  <div className="mt-1.5 grid gap-2 md:grid-cols-[minmax(0,1.4fr)_minmax(0,1fr)]">
                    <Field>
                      <FieldLabel>匹配表达式</FieldLabel>
                      <Input
                        value={rule.pattern}
                        onChange={(event) =>
                          updateMagic(rule.id, "pattern", event.target.value)
                        }
                        className="font-mono text-[12px]"
                      />
                    </Field>
                    <Field>
                      <FieldLabel>替换表达式</FieldLabel>
                      <Input
                        value={rule.replace}
                        onChange={(event) =>
                          updateMagic(rule.id, "replace", event.target.value)
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
              <div className="rounded-md bg-line-soft/50 px-3 py-2 text-[12px] text-muted-ink">
                {detectNote}
              </div>
            ) : null}
            <div className="rounded-md bg-line-soft/50 px-3 py-2.5">
              <SwitchField
                label="CloudSaver"
                description="需要服务器地址与账号；token 失效时自动重新登录。"
                checked={config.source.cloudsaver.enable}
                onCheckedChange={(checked) =>
                  updateSource("cloudsaver", "enable", checked)
                }
              />
              <div className="grid gap-2 md:grid-cols-2">
                <Field>
                  <FieldLabel>服务器</FieldLabel>
                  <Input
                    value={config.source.cloudsaver.server}
                    onChange={(event) =>
                      updateSource("cloudsaver", "server", event.target.value)
                    }
                    className="font-mono text-[12px]"
                  />
                </Field>
                <Field>
                  <FieldLabel>用户名</FieldLabel>
                  <Input
                    value={config.source.cloudsaver.username}
                    onChange={(event) =>
                      updateSource("cloudsaver", "username", event.target.value)
                    }
                  />
                </Field>
                <Field>
                  <FieldLabel>密码</FieldLabel>
                  <Input
                    type="password"
                    value={config.source.cloudsaver.password}
                    onChange={(event) =>
                      updateSource("cloudsaver", "password", event.target.value)
                    }
                  />
                </Field>
                <Field>
                  <FieldLabel>Token</FieldLabel>
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
            <div className="rounded-md bg-line-soft/50 px-3 py-2.5">
              <SwitchField
                label="PanSou"
                description="只需服务器地址；按关键词返回夸克网盘分享。"
                checked={config.source.pansou.enable}
                onCheckedChange={(checked) =>
                  updateSource("pansou", "enable", checked)
                }
              />
              <Field>
                <FieldLabel>服务器</FieldLabel>
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
      </div>

      <div className="-mb-4 sticky bottom-0 z-10 flex items-center justify-between gap-3 border-t border-line bg-canvas/95 py-2.5 backdrop-blur-sm lg:-mb-5">
        <span
          className={cn(
            "truncate text-[12px]",
            dirty ? "text-warn" : "text-muted-ink",
          )}
        >
          {dirty ? "有未保存的更改" : "保存后写入配置并重载调度"}
        </span>
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

      <Dialog
        open={qrOpen}
        onOpenChange={(open) => {
          setQrOpen(open);
          if (!open) {
            setQr(null);
            setQrState("waiting");
            setQrNote("");
            setQrNick("");
          }
        }}
      >
        <DialogContent className="w-[min(380px,calc(100vw-2rem))]">
          <DialogHeader>
            <DialogTitle>扫码登录夸克</DialogTitle>
            <DialogDescription>
              {qrState === "confirmed"
                ? "二维码已失效，Cookie 已写入服务器，可以关闭此窗口。"
                : "用夸克 App 扫一扫并在手机上确认，登录成功后会用该账号覆盖当前主账号。"}
            </DialogDescription>
          </DialogHeader>
          <DialogBody className="space-y-3">
            {qrState === "confirmed" ? (
              <div className="flex min-h-[184px] flex-col items-center justify-center gap-1.5 rounded-lg border border-line bg-panel px-4 text-center">
                <CheckCircle2 className="h-8 w-8 text-ok" strokeWidth={1.6} />
                <div className="text-[14px] font-medium text-ink">登录成功</div>
                <div className="text-[12px] leading-5 text-muted-ink">
                  {qrNick
                    ? `当前主账号已切换为 ${qrNick}`
                    : "当前主账号 Cookie 已更新"}
                </div>
              </div>
            ) : (
              <>
                <div className="flex justify-center">
                  <div className="rounded-lg border border-line bg-panel p-3">
                    {qr ? (
                      <QRCodeSVG value={qr.qr_url} size={196} marginSize={2} />
                    ) : (
                      <div className="grid h-[196px] w-[196px] place-items-center px-4 text-center text-[12px] text-muted-ink">
                        {startingQR
                          ? "正在获取二维码…"
                          : qrState === "expired"
                            ? "二维码已过期"
                            : qrNote || "没有可用的二维码"}
                      </div>
                    )}
                  </div>
                </div>
                <div
                  className={cn(
                    "flex items-center justify-center gap-2 text-[12px]",
                    qrState === "error" ? "text-danger" : "text-muted-ink",
                  )}
                >
                  {qr && qrState === "waiting" ? (
                    <>
                      <Loader2 className="h-3.5 w-3.5 animate-spin" />
                      等待扫码
                    </>
                  ) : (
                    <>
                      {qrState === "error" ? (
                        <AlertTriangle className="h-3.5 w-3.5" />
                      ) : null}
                      {qrNote || (startingQR ? "正在获取二维码…" : "等待扫码")}
                    </>
                  )}
                </div>
              </>
            )}
          </DialogBody>
          <DialogFooter>
            {qrState === "confirmed" ? null : (
              <Button
                variant="secondary"
                size="sm"
                disabled={startingQR}
                onClick={() => void openQR()}
              >
                {startingQR ? (
                  <Loader2 className="h-3.5 w-3.5 animate-spin" />
                ) : (
                  <RefreshCw className="h-3.5 w-3.5" />
                )}
                刷新二维码
              </Button>
            )}
            <Button size="sm" onClick={() => setQrOpen(false)}>
              关闭
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

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
