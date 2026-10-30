import { useRef, useState } from "react";
import { Loader2, LogIn } from "lucide-react";
import { BrandMark } from "../components/BrandMark";
import { Field, FieldLabel } from "../components/ui/field";
import { Button } from "../components/ui/button";
import { Input } from "../components/ui/input";
import { ApiError } from "../lib/api";

type LoginPageProps = {
  version: string;
  notice?: string;
  onLogin: (username: string, password: string) => Promise<void>;
};

export function LoginPage({ version, notice, onLogin }: LoginPageProps) {
  const [username, setUsername] = useState("admin");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  // 状态要等下一次渲染才生效，连点两下会绕过 busy 判断。
  const busyRef = useRef(false);

  const submit = async (event: React.FormEvent) => {
    event.preventDefault();
    if (busyRef.current) return;
    const name = username.trim();
    if (!name || !password) {
      setError("请输入用户名和密码");
      return;
    }
    busyRef.current = true;
    setBusy(true);
    setError("");
    try {
      await onLogin(name, password);
    } catch (cause) {
      setError(cause instanceof ApiError ? cause.message : "登录失败，请稍后重试");
    } finally {
      busyRef.current = false;
      setBusy(false);
    }
  };

  return (
    <div className="relative flex min-h-screen items-center justify-center overflow-hidden bg-canvas px-4">
      <div
        aria-hidden
        className="pointer-events-none absolute inset-x-0 -top-48 h-[520px] bg-[radial-gradient(60%_100%_at_50%_0%,var(--brand-soft),transparent_70%)]"
      />
      <div className="auth-enter relative w-full max-w-[380px]">
        <div className="mb-5 flex items-center gap-2.5">
          <BrandMark className="h-9 w-9" />
          <div>
            <div className="text-[15px] font-semibold tracking-wide text-ink">
              夸克自动转存
            </div>
            <div className="font-mono text-[12px] tracking-[0.14em] text-subtle">
              QAS · SELF-HOSTED
            </div>
          </div>
        </div>

        {notice ? (
          <div
            role="status"
            className="mb-3 flex items-center gap-2 rounded-md bg-warn-soft px-3 py-2 text-[12px] text-warn"
          >
            {notice}
          </div>
        ) : null}

        <form
          onSubmit={submit}
          className="rounded-panel border border-line bg-panel/95 p-5 shadow-panel backdrop-blur-sm"
        >
          <div className="mb-4">
            <h1 className="text-[16px] font-semibold tracking-[-0.01em] text-ink">登录控制台</h1>
            <p className="mt-1 text-[12px] leading-5 text-muted-ink">
              使用配置文件里的 WebUI 账号。首次启动时随机密码会打印在服务端日志里。
            </p>
          </div>
          <div className="space-y-3">
            <Field>
              <FieldLabel htmlFor="login-username">用户名</FieldLabel>
              <Input
                id="login-username"
                name="username"
                autoComplete="username"
                value={username}
                onChange={(event) => {
                  setUsername(event.target.value);
                  setError("");
                }}
                aria-invalid={Boolean(error)}
                aria-describedby={error ? "login-error" : undefined}
                placeholder="admin"
              />
            </Field>
            <Field>
              <FieldLabel htmlFor="login-password">密码</FieldLabel>
              <Input
                id="login-password"
                name="password"
                type="password"
                autoComplete="current-password"
                value={password}
                onChange={(event) => {
                  setPassword(event.target.value);
                  setError("");
                }}
                aria-invalid={Boolean(error)}
                aria-describedby={error ? "login-error" : undefined}
                placeholder="••••••••"
              />
            </Field>
          </div>
          {error ? (
            <p id="login-error" role="alert" className="mt-3 text-[12px] text-danger">
              {error}
            </p>
          ) : null}
          <Button type="submit" className="mt-4 w-full" disabled={busy}>
            {busy ? (
              <Loader2 className="h-3.5 w-3.5 animate-spin" />
            ) : (
              <LogIn className="h-3.5 w-3.5" />
            )}
            {busy ? "正在登录" : "登录"}
          </Button>
        </form>

        <div className="mt-3 flex items-center justify-between font-mono text-[12px] text-subtle">
          <span>quark-auto-save · 自建实例</span>
          <span>{version || "—"}</span>
        </div>
      </div>
    </div>
  );
}
