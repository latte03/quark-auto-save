import { useState } from "react";
import { Loader2, LogIn } from "lucide-react";
import { Field } from "../components/ui/field";
import { Button } from "../components/ui/button";
import { Input } from "../components/ui/input";

type LoginPageProps = {
  version: string;
  onLogin: (username: string, password: string) => Promise<void>;
};

export function LoginPage({ version, onLogin }: LoginPageProps) {
  const [username, setUsername] = useState("admin");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  const submit = async (event: React.FormEvent) => {
    event.preventDefault();
    if (busy) return;
    setBusy(true);
    setError("");
    try {
      await onLogin(username.trim(), password);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "登录失败");
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="flex min-h-screen items-center justify-center bg-canvas px-4">
      <div className="w-full max-w-[380px]">
        <div className="mb-5 flex items-center gap-2.5">
          <div className="grid h-9 w-9 place-items-center rounded-lg bg-brand text-[15px] font-bold text-white">
            夸
          </div>
          <div>
            <div className="text-[15px] font-semibold tracking-wide text-ink">
              夸克自动转存
            </div>
            <div className="font-mono text-[12px] tracking-[0.14em] text-subtle">
              QAS · SELF-HOSTED
            </div>
          </div>
        </div>

        <form
          onSubmit={submit}
          className="rounded-panel border border-line bg-panel p-5 shadow-sm"
        >
          <div className="mb-4">
            <h1 className="text-[14px] font-semibold text-ink">登录控制台</h1>
            <p className="mt-1 text-[12px] leading-5 text-muted">
              使用配置文件里的 WebUI 账号。首次启动时随机密码会打印在服务端日志里。
            </p>
          </div>
          <div className="space-y-3">
            <Field label="用户名" htmlFor="login-username">
              <Input
                id="login-username"
                name="username"
                autoComplete="username"
                value={username}
                onChange={(event) => setUsername(event.target.value)}
                placeholder="admin"
              />
            </Field>
            <Field label="密码" htmlFor="login-password">
              <Input
                id="login-password"
                name="password"
                type="password"
                autoComplete="current-password"
                value={password}
                onChange={(event) => setPassword(event.target.value)}
                placeholder="••••••••"
              />
            </Field>
          </div>
          {error ? (
            <p role="alert" className="mt-3 text-[12px] text-danger">
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
