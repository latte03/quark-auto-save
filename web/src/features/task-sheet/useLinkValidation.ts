import { useEffect, useState } from "react";
import { validateShare } from "../../lib/api";
import type { Task } from "../../lib/types";
import { useToast } from "../../components/ToastProvider";

export type LinkState = "unknown" | "ok" | "bad";

export function useLinkValidation(open: boolean, task: Task | null) {
  const notify = useToast();
  const [state, setState] = useState<LinkState>("unknown");
  const [validating, setValidating] = useState(false);

  useEffect(() => {
    if (!open) return;
    setState(task?.shareurl_ban ? "bad" : task?.shareurl ? "ok" : "unknown");
  }, [open, task]);

  const validate = async (shareurl: string) => {
    if (!shareurl.trim()) {
      notify("请先填写分享链接", "warn");
      return;
    }
    setValidating(true);
    try {
      const result = await validateShare(shareurl);
      if (result.ok) {
        setState("ok");
        notify(
          `链接有效：${result.title || "未命名分享"} · ${result.count ?? 0} 项`,
          "ok",
        );
      } else {
        setState("bad");
        notify(result.message || "链接无效或已失效", "warn");
      }
    } catch (cause) {
      setState("bad");
      notify(cause instanceof Error ? cause.message : "验证失败", "error");
    } finally {
      setValidating(false);
    }
  };

  return {
    state,
    validating,
    validate,
    reset: () => setState("unknown"),
    markOk: () => setState("ok"),
  };
}

export type LinkValidation = ReturnType<typeof useLinkValidation>;
