import { useCallback, useEffect, useState } from "react";
import {
  AlertTriangle,
  CheckCircle2,
  KeyRound,
  Loader2,
  Plus,
  QrCode,
  RefreshCw,
  Trash2,
  Zap,
} from "lucide-react";
import { QRCodeSVG } from "qrcode.react";
import {
  getQuarkAccount,
  pollQuarkQR,
  startQuarkQR,
  type QuarkAccount,
  type QuarkQR,
} from "../../lib/api";
import { cn } from "../../lib/utils";
import { formatTime } from "../../lib/format";
import { Badge } from "../../components/ui/badge";
import { Button } from "../../components/ui/button";
import {
  Dialog,
  DialogBody,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "../../components/ui/dialog";
import { Textarea } from "../../components/ui/textarea";
import { PageSection } from "../../components/PageSection";
import { useToast } from "../../components/ToastProvider";
import { useSettings } from "./SettingsContext";

type QRState = "waiting" | "confirmed" | "expired" | "error";

export function CookieSection() {
  const { config, update, running, onSign, onAdoptPatch } = useSettings();
  const notify = useToast();
  const [account, setAccount] = useState<QuarkAccount | null>(null);
  const [checkingAccount, setCheckingAccount] = useState(false);
  const [qrOpen, setQrOpen] = useState(false);
  const [qr, setQr] = useState<QuarkQR | null>(null);
  const [qrState, setQrState] = useState<QRState>("waiting");
  const [qrNote, setQrNote] = useState("");
  const [qrNick, setQrNick] = useState("");
  const [startingQR, setStartingQR] = useState(false);

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
              // 服务端已经把 Cookie 落盘，基线与草稿一起更新，不制造「未保存」。
              onAdoptPatch({ cookie: next.cookies });
            }
            setAccount({
              ok: true,
              nickname: next.nickname,
              checked_at: new Date().toISOString(),
            });
            notify(
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
  }, [qrOpen, qr, qrState, onAdoptPatch, notify]);

  return (
    <PageSection
      id="settings-cookie"
      title="Cookie"
      description="第一个账号执行转存，所有账号执行签到。可以扫码登录直接写入主账号，也可以手动粘贴 Cookie。"
      action={
        <div className="flex gap-2">
          <Button
            variant="secondary"
            size="sm"
            disabled={startingQR}
            onClick={() => void openQR()}
          >
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
            onClick={() => update("cookie", [...config.cookie, ""])}
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
                校验于 {formatTime(account.checked_at)}
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
    </PageSection>
  );
}
