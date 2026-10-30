import { useEffect, useRef, useState } from "react";
import { validateShare } from "../../lib/api";
import type { Task } from "../../lib/types";
import { useToast } from "../../components/ToastProvider";

export type LinkState = "unknown" | "ok" | "bad";

export function useLinkValidation(open: boolean, task: Task | null) {
  const notify = useToast();
  const [state, setState] = useState<LinkState>("unknown");
  const [validating, setValidating] = useState(false);
  // 多次校验可能同时在飞（连点、换任务后旧请求晚到），只认最新一次的结果。
  const seq = useRef(0);

  useEffect(() => {
    if (!open) return;
    seq.current += 1;
    setValidating(false);
    setState(task?.shareurl_ban ? "bad" : task?.shareurl ? "ok" : "unknown");
  }, [open, task]);

  const validate = async (shareurl: string) => {
    // 先作废在飞请求：清空链接后再点验证，旧结果不该再写回状态。
    const current = (seq.current += 1);
    if (!shareurl.trim()) {
      setValidating(false);
      notify("请先填写分享链接", "warn");
      return;
    }
    setValidating(true);
    try {
      const result = await validateShare(shareurl);
      if (current !== seq.current) return;
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
      if (current !== seq.current) return;
      setState("bad");
      notify(cause instanceof Error ? cause.message : "验证失败", "error");
    } finally {
      if (current === seq.current) setValidating(false);
    }
  };

  return {
    state,
    validating,
    validate,
    // 手动改写状态同样要作废在飞校验，否则旧结果会把新状态覆盖掉。
    reset: () => {
      seq.current += 1;
      setValidating(false);
      setState("unknown");
    },
    markOk: () => {
      seq.current += 1;
      setValidating(false);
      setState("ok");
    },
  };
}

export type LinkValidation = ReturnType<typeof useLinkValidation>;
