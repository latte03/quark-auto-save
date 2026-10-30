import { useEffect, useRef, useState } from "react";
import {
  Check,
  CircleHelp,
  Loader2,
  Pencil,
  Plus,
  Trash2,
  X,
} from "lucide-react";
import { getConfig, previewRename, renameMagicRule } from "../../lib/api";
import { Button } from "../../components/ui/button";
import { Field, FieldLabel } from "../../components/ui/field";
import {
  HoverCard,
  HoverCardContent,
  HoverCardTrigger,
} from "../../components/ui/hover-card";
import { Input } from "../../components/ui/input";
import { PageSection } from "../../components/PageSection";
import { useToast } from "../../components/ToastProvider";
import { magicVariables, newRuleId } from "./constants";
import { useSettings } from "./SettingsContext";

export function MagicSection() {
  const { config, setConfig, update, updateMagic, onTasksReload } = useSettings();
  const notify = useToast();
  const [sample, setSample] = useState("Black.Mirror.S07E05.mkv");
  const [renamingKey, setRenamingKey] = useState<string | null>(null);
  const [renameDraft, setRenameDraft] = useState("");
  const [renamingBusy, setRenamingBusy] = useState(false);
  const [previews, setPreviews] = useState<Record<string, string>>({});
  // renamingBusy 要等渲染后才生效，回车连按两下会重复提交，用 ref 做同步锁。
  const renamingRef = useRef(false);

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

  // Renaming only touches the display name; tasks reference the stable id.
  const commitRename = async (id: string) => {
    if (renamingRef.current) return;
    const current = config.magic_regex.find((rule) => rule.id === id);
    const to = renameDraft.trim();
    if (!current || !to || to === current.name) {
      setRenamingKey(null);
      return;
    }
    if (config.magic_regex.some((rule) => rule.id !== id && rule.name === to)) {
      notify(`已存在规则 ${to}`, "warn");
      return;
    }
    renamingRef.current = true;
    setRenamingBusy(true);
    try {
      await renameMagicRule(id, to);
      setConfig(await getConfig());
      setRenamingKey(null);
      notify(`已重命名为 ${to}`, "ok");
      onTasksReload();
    } catch (cause) {
      notify(cause instanceof Error ? cause.message : "重命名失败", "error");
    } finally {
      renamingRef.current = false;
      setRenamingBusy(false);
    }
  };

  return (
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
            notify(`已添加 ${name}`, "ok");
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
                        if (event.nativeEvent.isComposing || renamingBusy) return;
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
                  <FieldLabel htmlFor={`magic-pattern-${rule.id}`}>匹配表达式</FieldLabel>
                  <Input
                    id={`magic-pattern-${rule.id}`}
                    value={rule.pattern}
                    onChange={(event) =>
                      updateMagic(rule.id, "pattern", event.target.value)
                    }
                    className="font-mono text-[12px]"
                  />
                </Field>
                <Field>
                  <FieldLabel htmlFor={`magic-replace-${rule.id}`}>替换表达式</FieldLabel>
                  <Input
                    id={`magic-replace-${rule.id}`}
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
  );
}
