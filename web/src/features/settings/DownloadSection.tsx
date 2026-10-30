import { useEffect, useState } from "react";
import {
  AlertTriangle,
  CheckCircle2,
  CircleAlert,
  FolderOpen,
  HardDrive,
  Loader2,
  Plus,
} from "lucide-react";
import { getAuthorizedDirs, validateLocalDir, type LocalDirCheck } from "../../lib/api";
import { pickSharedDirectory } from "../../lib/trim";
import { cn } from "../../lib/utils";
import { Button } from "../../components/ui/button";
import { Field, FieldDescription, FieldLabel } from "../../components/ui/field";
import { Input } from "../../components/ui/input";
import { PageSection } from "../../components/PageSection";
import { useToast } from "../../components/ToastProvider";
import { LocalDirDialog } from "../LocalDirDialog";
import { formatFree } from "./constants";
import { useSettings } from "./SettingsContext";

export function DownloadSection() {
  const { config, updateLocalDownload, onAdoptDefaults } = useSettings();
  const notify = useToast();
  const [pickingDir, setPickingDir] = useState(false);
  const [checkingDir, setCheckingDir] = useState(false);
  const [dirCheck, setDirCheck] = useState<LocalDirCheck | null>(null);
  const [authorizedDirs, setAuthorizedDirs] = useState<string[]>([]);
  const [authorizedError, setAuthorizedError] = useState("");
  const [loadingAuthorized, setLoadingAuthorized] = useState(false);
  const [dirPickerOpen, setDirPickerOpen] = useState(false);

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
      notify(cause instanceof Error ? cause.message : "目录检查失败", "error");
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
      notify(
        cause instanceof Error ? cause.message : "无法打开飞牛目录选择器",
        "warn",
      );
    } finally {
      setPickingDir(false);
    }
  };

  useEffect(() => {
    void loadAuthorizedDirs();
    // Reads the authorization list once when the settings page mounts.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return (
    <PageSection
      id="settings-download"
      title="下载"
      description="任务开启「下载到 NAS」后，云端保存目录里本地没有的文件会拉取到这里；目录须位于存储空间下，且应用用户可写。"
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
                onBlur={() => void checkDirectory(config.local_download.dir)}
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
        <div className="grid gap-3 md:grid-cols-2">
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
        </div>
      </div>

      <LocalDirDialog
        open={dirPickerOpen}
        initial={config.local_download.dir}
        onOpenChange={setDirPickerOpen}
        onSelect={(path) => {
          updateLocalDownload("dir", path);
          void checkDirectory(path);
          notify(`下载目录已设为 ${path}`, "ok");
        }}
      />
    </PageSection>
  );
}
