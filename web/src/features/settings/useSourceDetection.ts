import { useEffect, useRef, useState, type SetStateAction } from "react";
import { detectSources, type SourceDetection } from "../../lib/api";
import type { AppConfig } from "../../lib/types";
import { pluginLabels } from "../../lib/mock";

export type Detection = {
  searchBusy: boolean;
  alistBusy: boolean;
  searchNote: string;
  alistNote: string;
  runSearch: () => Promise<void>;
  runAlist: () => Promise<void>;
};

type DetectionParams = {
  config: AppConfig;
  setConfig: (updater: SetStateAction<AppConfig>) => void;
  updateSource: (
    source: "cloudsaver" | "pansou",
    key: string,
    value: string | boolean,
  ) => void;
  onAdoptDefaults: () => void;
};

const ALIST_PLUGINS = ["alist", "alist_strm_gen", "alist_sync"] as const;

export function useSourceDetection({
  config,
  setConfig,
  updateSource,
  onAdoptDefaults,
}: DetectionParams): Detection {
  const [searchBusy, setSearchBusy] = useState(false);
  const [alistBusy, setAlistBusy] = useState(false);
  const [searchNote, setSearchNote] = useState("");
  const [alistNote, setAlistNote] = useState("");
  const autoRan = useRef(false);

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
      setSearchNote(
        "未在本机 8008 / 50080 端口发现 CloudSaver 或 PanSou，请手动填写服务器地址。",
      );
    } else if (filled > 0) {
      setSearchNote(`已识别本机 ${found.join("、")}，已填入并启用，保存后生效。`);
    } else {
      setSearchNote(`已识别本机 ${found.join("、")}（已有配置，未覆盖）。`);
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
    const targets = ALIST_PLUGINS.filter(
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

  const detect = async (scope: "search" | "alist" | "all", adopt = false) => {
    if (scope !== "alist") setSearchBusy(true);
    if (scope !== "search") setAlistBusy(true);
    try {
      const result = await detectSources();
      if (scope !== "alist") fillSearchSources(result);
      if (scope !== "search") fillAlistServers(result);
      if (adopt) onAdoptDefaults();
    } catch (cause) {
      const message = cause instanceof Error ? cause.message : "自动识别失败";
      if (scope !== "alist") setSearchNote(message);
      if (scope !== "search") setAlistNote(message);
    } finally {
      if (scope !== "alist") setSearchBusy(false);
      if (scope !== "search") setAlistBusy(false);
    }
  };

  useEffect(() => {
    if (autoRan.current) return;
    autoRan.current = true;
    const searchEmpty =
      !config.source.cloudsaver.server.trim() &&
      !config.source.pansou.server.trim();
    const alistEmpty = ALIST_PLUGINS.every(
      (plugin) => !String(config.plugins[plugin]?.url ?? "").trim(),
    );
    if (searchEmpty || alistEmpty) {
      void detect(searchEmpty ? (alistEmpty ? "all" : "search") : "alist", true);
    }
    // Detection probes local ports once per settings session.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return {
    searchBusy,
    alistBusy,
    searchNote,
    alistNote,
    runSearch: () => detect("search"),
    runAlist: () => detect("alist"),
  };
}
