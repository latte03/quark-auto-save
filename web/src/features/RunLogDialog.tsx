import { useEffect, useRef, useState } from "react";
import { Check, Copy, Loader2, RotateCw } from "lucide-react";
import { Button } from "../components/ui/button";
import {
  Dialog,
  DialogBody,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "../components/ui/dialog";
import { Badge } from "../components/ui/badge";
import { canCopyText } from "../lib/clipboard";

function lineClass(line: string) {
  if (line.includes("❌") || line.includes("失败") || line.includes("失效")) {
    return "text-red-300";
  }
  if (line.includes("✅") || line.includes("成功")) {
    return "text-emerald-300";
  }
  if (line.includes("📢") || line.includes("推送")) {
    return "text-amber-200";
  }
  if (line.startsWith("#") || line.startsWith("===")) {
    return "text-slate-400";
  }
  if (line.includes("→")) {
    return "text-sky-200";
  }
  return "text-slate-300";
}

export function RunLogDialog({
  open,
  onOpenChange,
  lines,
  error,
  loading,
  running,
  dry,
  allowRerun = true,
  onRunAgain,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  lines: string[];
  error?: string;
  loading?: boolean;
  running: boolean;
  dry?: boolean;
  allowRerun?: boolean;
  onRunAgain: () => void;
}) {
  const bottomRef = useRef<HTMLDivElement>(null);
  const scrollRef = useRef<HTMLDivElement>(null);
  const stickRef = useRef(true);
  const copiedTimer = useRef<number | null>(null);
  const [copied, setCopied] = useState(false);
  const canCopy = canCopyText();

  useEffect(() => {
    if (!open) return;
    stickRef.current = true;
  }, [open]);

  useEffect(() => {
    if (!open || !stickRef.current) return;
    bottomRef.current?.scrollIntoView({ block: "end" });
  }, [lines, open]);

  useEffect(
    () => () => {
      if (copiedTimer.current) window.clearTimeout(copiedTimer.current);
    },
    [],
  );

  const copyAll = async () => {
    if (!canCopy) return;
    try {
      await navigator.clipboard.writeText(lines.join("\n"));
    } catch {
      return;
    }
    setCopied(true);
    if (copiedTimer.current) window.clearTimeout(copiedTimer.current);
    copiedTimer.current = window.setTimeout(() => setCopied(false), 1600);
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="w-[min(820px,calc(100vw-2rem))]">
        <DialogHeader>
          <div className="flex items-center gap-2">
            <DialogTitle>{dry ? "运行日志 · 试运行" : "运行日志"}</DialogTitle>
            <Badge variant={running ? "info" : "ok"} dot>
              {running ? "运行中" : "已完成"}
            </Badge>
          </div>
          <DialogDescription>
            实时输出，来自 Go 服务端运行器；关闭窗口不会中断运行。
            {dry ? "试运行不写入网盘，也不会计入运行记录。" : ""}
          </DialogDescription>
        </DialogHeader>
        <DialogBody className="p-0">
          {error ? (
            <div
              role="alert"
              className="border-b border-danger/30 bg-danger-soft/60 px-4 py-2 text-[12px] text-danger"
            >
              {error}
            </div>
          ) : null}
          <div className="flex items-center justify-between border-b border-slate-800 bg-[#0b1220] px-4 py-2">
            <div className="flex items-center gap-2 font-mono text-[12px] tracking-[0.06em] text-slate-400">
              <span
                className={`h-1.5 w-1.5 rounded-full ${running ? "bg-emerald-400" : "bg-slate-500"}`}
              />
              qas serve · run
            </div>
            <span className="font-mono text-[12px] text-slate-400">
              {running ? "streaming" : `${lines.length} lines`}
            </span>
          </div>
          <div
            ref={scrollRef}
            onScroll={() => {
              const el = scrollRef.current;
              if (!el) return;
              stickRef.current =
                el.scrollHeight - el.scrollTop - el.clientHeight < 24;
            }}
            className="scrollbar-dark h-[420px] overflow-y-auto bg-[#0b1220] px-4 py-3 font-mono text-[12px] leading-[1.7]"
          >
            {lines.length === 0 ? (
              <div className="flex h-full items-center justify-center text-slate-400">
                {loading ? (
                  <>
                    <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                    正在读取日志…
                  </>
                ) : running ? (
                  <>
                    <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                    等待输出…
                  </>
                ) : (
                  "还没有运行记录"
                )}
              </div>
            ) : (
              lines.map((line, index) => (
                <div key={`${index}-${line.slice(0, 12)}`} className={lineClass(line)}>
                  {line || "\u00a0"}
                </div>
              ))
            )}
            {running ? (
              <div className="mt-1 flex items-center gap-2 text-slate-400">
                <Loader2 className="h-3 w-3 animate-spin" />
                正在等待下一个任务…
              </div>
            ) : null}
            <div ref={bottomRef} />
          </div>
        </DialogBody>
        <DialogFooter className="justify-between">
          <Button
            variant="ghost"
            size="sm"
            // aria-disabled 保留焦点，鼠标悬停或聚焦时还能看到不能复制的原因。
            aria-disabled={!canCopy || undefined}
            aria-label={canCopy ? "复制日志" : "当前地址不支持复制"}
            title={canCopy ? undefined : "当前地址不支持复制"}
            className={canCopy ? undefined : "opacity-50"}
            onClick={() => void copyAll()}
          >
            {copied ? <Check className="h-3.5 w-3.5 text-ok" /> : <Copy className="h-3.5 w-3.5" />}
            {copied ? "已复制" : "复制日志"}
          </Button>
          <div className="flex items-center gap-2">
            <Button variant="secondary" size="sm" onClick={() => onOpenChange(false)}>
              关闭
            </Button>
            {allowRerun ? (
              <Button size="sm" onClick={onRunAgain} disabled={running}>
                {running ? (
                  <Loader2 className="h-3.5 w-3.5 animate-spin" />
                ) : (
                  <RotateCw className="h-3.5 w-3.5" />
                )}
                再次运行
              </Button>
            ) : null}
          </div>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
