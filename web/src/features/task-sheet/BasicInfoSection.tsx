import { useId, useState } from "react";
import {
  FolderOpen,
  Link2,
  Loader2,
  Plus,
  Search,
  TriangleAlert,
} from "lucide-react";
import { pickSharedDirectory } from "../../lib/trim";
import { Badge } from "../../components/ui/badge";
import { Button } from "../../components/ui/button";
import {
  Field,
  FieldDescription,
  FieldLabel,
} from "../../components/ui/field";
import { Input } from "../../components/ui/input";
import { SwitchField } from "../../components/SwitchField";
import { useToast } from "../../components/ToastProvider";
import { LocalDirDialog } from "../LocalDirDialog";
import { sectionTitle } from "./constants";
import { useTaskDraftContext } from "./TaskDraftContext";

export function BasicInfoSection() {
  const {
    draft,
    update,
    search,
    link,
    searchReady,
    onOpenSettings,
    openPicker,
  } = useTaskDraftContext();
  const notify = useToast();
  const [pickingLocal, setPickingLocal] = useState(false);
  const [localPickerOpen, setLocalPickerOpen] = useState(false);
  const downloadDirId = useId();

  const pickLocalDirectory = async () => {
    setPickingLocal(true);
    try {
      const dir = await pickSharedDirectory();
      if (!dir) return;
      update("local_download", {
        ...(draft.local_download ?? { enable: true }),
        enable: true,
        dir,
      });
    } catch (cause) {
      notify(
        cause instanceof Error ? cause.message : "无法打开飞牛目录选择器",
        "warn",
      );
    } finally {
      setPickingLocal(false);
    }
  };

  return (
    <section className="space-y-3">
      {draft.shareurl_ban ? (
        <div className="flex items-start gap-2 rounded-md border border-danger/20 bg-danger-soft px-3 py-2.5 text-danger">
          <TriangleAlert className="mt-0.5 h-4 w-4 shrink-0" />
          <div className="min-w-0 flex-1 text-[12px] leading-5">
            <b className="font-semibold">该分享链接已失效</b>
            <div className="opacity-80">{draft.shareurl_ban}</div>
          </div>
          <Button
            variant="destructive"
            size="sm"
            onClick={() => search.setShow(true)}
          >
            查找替换
          </Button>
        </div>
      ) : null}

      <div className="flex items-center justify-between">
        <h3 className={sectionTitle}>基本信息</h3>
        <Badge
          variant={
            link.state === "bad" ? "danger" : link.state === "ok" ? "ok" : "neutral"
          }
        >
          {link.state === "bad"
            ? "链接异常"
            : link.state === "ok"
              ? "链接有效"
              : "未验证"}
        </Badge>
      </div>
      <Field>
        <FieldLabel htmlFor="taskname">任务名称</FieldLabel>
        <Input
          id="taskname"
          value={draft.taskname}
          onChange={(event) => update("taskname", event.target.value)}
          placeholder="例如：黑镜"
        />
      </Field>
      <Field>
        <FieldLabel htmlFor="shareurl">分享链接</FieldLabel>
        <div className="flex items-center gap-1.5">
          <Input
            id="shareurl"
            value={draft.shareurl}
            onChange={(event) => {
              update("shareurl", event.target.value);
              link.reset();
            }}
            placeholder="https://pan.quark.cn/s/..."
          />
          <Button
            variant="secondary"
            size="icon"
            aria-label="验证链接"
            disabled={link.validating}
            onClick={() => void link.validate(draft.shareurl)}
          >
            {link.validating ? (
              <Loader2 className="h-3.5 w-3.5 animate-spin" />
            ) : (
              <Link2 className="h-3.5 w-3.5" />
            )}
          </Button>
          <Button
            variant="secondary"
            size="sm"
            onClick={() => search.setShow(true)}
          >
            <Search className="h-3.5 w-3.5" />
            搜索
          </Button>
        </div>
        <FieldDescription>
          支持子目录链接与提取码链接。链接失效时，任务会被标记并跳过转存。
        </FieldDescription>
      </Field>

      {search.show ? (
        <div className="rounded-md border border-line bg-line-soft/40 p-3">
          {!searchReady ? (
            <div className="flex items-start gap-2.5">
              <TriangleAlert className="mt-0.5 h-4 w-4 shrink-0 text-warn" />
              <div className="min-w-0 flex-1">
                <div className="text-[12px] font-medium text-ink">
                  还没有可用的搜索源
                </div>
                <div className="mt-0.5 text-[12px] text-muted-ink">
                  先到设置里启用 PanSou 或 CloudSaver；装在本机时可以直接点「自动识别」。
                </div>
                <div className="mt-2.5 flex items-center gap-2">
                  <Button size="sm" onClick={onOpenSettings}>
                    去设置搜索源
                  </Button>
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => search.setShow(false)}
                  >
                    收起
                  </Button>
                </div>
              </div>
            </div>
          ) : (
            <>
              <div className="flex items-center gap-2">
                <Search className="h-3.5 w-3.5 text-subtle" />
                <Input
                  value={search.query}
                  onChange={(event) => search.setQuery(event.target.value)}
                  placeholder="搜索资源，例如：黑镜"
                  aria-label="搜索资源"
                  className="h-7"
                />
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => search.setShow(false)}
                >
                  收起
                </Button>
              </div>
              <div className="mt-2 space-y-1.5">
                {search.searching ? (
                  <div className="flex items-center gap-2 px-1 py-2 text-[12px] text-subtle">
                    <Loader2 className="h-3.5 w-3.5 animate-spin" />
                    正在搜索…
                  </div>
                ) : null}
                {!search.searching && search.messages.length > 0 ? (
                  <div className="rounded border border-warn/30 bg-warn-soft px-2.5 py-1.5 text-[12px] text-warn">
                    {search.messages.join("；")}
                  </div>
                ) : null}
                {!search.searching &&
                search.query.trim() &&
                search.hits.length === 0 &&
                search.messages.length === 0 ? (
                  <div className="px-1 py-2 text-[12px] text-subtle">
                    没有找到相关分享。
                  </div>
                ) : null}
                {!search.searching && search.hits.length > 0 ? (
                  <div className="px-1 text-[12px] text-subtle">
                    点击候选可预览分享文件，再决定是否使用。
                  </div>
                ) : null}
                {search.hits.map((hit) => (
                  <button
                    key={hit.shareurl}
                    type="button"
                    title="点击预览分享文件"
                    className="flex w-full items-start gap-2 rounded border border-transparent bg-panel px-2.5 py-2 text-left transition-colors hover:border-brand/30 hover:bg-brand-soft/40"
                    onClick={() => search.setPreviewHit(hit)}
                  >
                    <span className="mt-1 h-1.5 w-1.5 shrink-0 rounded-full bg-ok" />
                    <span className="min-w-0 flex-1">
                      <span className="block truncate text-[12px] font-medium text-ink">
                        {hit.taskname}
                      </span>
                      <span className="mt-0.5 block truncate text-[12px] text-muted-ink">
                        {hit.content}
                      </span>
                    </span>
                    <span className="shrink-0 font-mono text-[12px] text-subtle">
                      {hit.channel || hit.source}
                    </span>
                  </button>
                ))}
              </div>
            </>
          )}
        </div>
      ) : null}

      <Field>
        <FieldLabel htmlFor="savepath">保存路径</FieldLabel>
        <div className="flex items-center gap-1.5">
          <Input
            id="savepath"
            value={draft.savepath}
            onChange={(event) => update("savepath", event.target.value)}
            placeholder="/video/tv/..."
          />
          <Button
            variant="secondary"
            size="sm"
            onClick={() => openPicker("savepath")}
          >
            <FolderOpen className="h-3.5 w-3.5" />
            选择
          </Button>
        </div>
      </Field>
      <SwitchField
        label="下载到 NAS"
        description="转存后把云端保存目录里本地缺少的文件拉取到本地目录。"
        checked={Boolean(draft.local_download?.enable)}
        onCheckedChange={(checked) =>
          update("local_download", {
            ...(draft.local_download ?? {}),
            enable: checked,
          })
        }
      />
      {draft.local_download?.enable ? (
        <>
          <Field>
            <FieldLabel htmlFor={downloadDirId}>下载目录</FieldLabel>
            <div className="flex items-center gap-1.5">
              <Input
                id={downloadDirId}
                value={draft.local_download?.dir ?? ""}
                onChange={(event) =>
                  update("local_download", {
                    enable: true,
                    ...(draft.local_download ?? {}),
                    dir: event.target.value,
                  })
                }
                placeholder="/vol2/1000/影视"
              />
              <Button
                variant="secondary"
                size="sm"
                onClick={() => setLocalPickerOpen(true)}
              >
                <FolderOpen className="h-3.5 w-3.5" />
                选择目录
              </Button>
              <Button
                variant="ghost"
                size="sm"
                disabled={pickingLocal}
                onClick={() => void pickLocalDirectory()}
              >
                {pickingLocal ? (
                  <Loader2 className="h-3.5 w-3.5 animate-spin" />
                ) : (
                  <Plus className="h-3.5 w-3.5" />
                )}
                新增授权
              </Button>
            </div>
            <FieldDescription>留空使用设置里的默认目录。</FieldDescription>
          </Field>
          <SwitchField
            label="覆盖同名文件"
            description="本地已存在同名文件时重新下载；关闭时保留已有文件。"
            checked={Boolean(draft.local_download?.overwrite)}
            onCheckedChange={(checked) =>
              update("local_download", {
                enable: true,
                ...(draft.local_download ?? {}),
                overwrite: checked,
              })
            }
          />
        </>
      ) : null}

      <LocalDirDialog
        open={localPickerOpen}
        initial={draft.local_download?.dir}
        onOpenChange={setLocalPickerOpen}
        onSelect={(path) => {
          update("local_download", {
            enable: true,
            ...(draft.local_download ?? {}),
            dir: path,
          });
          notify(`下载目录已设为 ${path}`, "ok");
        }}
      />
    </section>
  );
}
