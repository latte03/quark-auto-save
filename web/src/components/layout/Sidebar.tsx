import {
  FolderOpen,
  LayoutDashboard,
  ListVideo,
  LogOut,
  ScrollText,
  Settings2,
  X,
} from "lucide-react";
import type { Page } from "../../lib/types";
import { cn } from "../../lib/utils";

const navItems: { page: Page; label: string; icon: typeof LayoutDashboard }[] = [
  { page: "overview", label: "总览", icon: LayoutDashboard },
  { page: "tasks", label: "任务", icon: ListVideo },
  { page: "files", label: "文件", icon: FolderOpen },
  { page: "settings", label: "设置", icon: Settings2 },
  { page: "runs", label: "运行记录", icon: ScrollText },
];

type SidebarProps = {
  page: Page;
  taskCount: number;
  bannedCount: number;
  username: string;
  version: string;
  mobileOpen: boolean;
  onNavigate: (page: Page) => void;
  onLogout: () => void;
  onClose: () => void;
};

export function Sidebar({
  page,
  taskCount,
  bannedCount,
  username,
  version,
  mobileOpen,
  onNavigate,
  onLogout,
  onClose,
}: SidebarProps) {
  return (
    <>
      {mobileOpen ? (
        <button
          aria-label="关闭导航"
          className="overlay-in fixed inset-0 z-30 bg-slate-950/40 lg:hidden"
          onClick={onClose}
        />
      ) : null}
      <aside
        className={cn(
          "fixed inset-y-0 left-0 z-40 flex w-[232px] shrink-0 flex-col overflow-y-auto bg-sidebar text-white transition-transform duration-200 lg:sticky lg:inset-y-auto lg:top-0 lg:h-dvh lg:translate-x-0",
          mobileOpen ? "translate-x-0" : "-translate-x-full",
        )}
        aria-label="主导航"
      >
        <div className="flex items-center gap-2.5 px-4 pb-5 pt-5">
          <div className="grid h-8 w-8 place-items-center rounded-lg bg-brand text-[14px] font-bold text-white">
            夸
          </div>
          <div className="min-w-0 flex-1">
            <div className="truncate text-[14px] font-semibold tracking-wide text-white">
              夸克自动转存
            </div>
            <div className="font-mono text-[12px] tracking-[0.14em] text-slate-400">
              QAS · SELF-HOSTED
            </div>
          </div>
          <button
            aria-label="关闭导航"
            onClick={onClose}
            className="grid h-7 w-7 place-items-center rounded-md text-slate-400 hover:bg-white/10 hover:text-white lg:hidden"
          >
            <X className="h-4 w-4" />
          </button>
        </div>

        <nav className="flex flex-1 flex-col gap-0.5 px-2.5">
          {navItems.map((item) => {
            const Icon = item.icon;
            const active = page === item.page;
            return (
              <button
                key={item.page}
                onClick={() => {
                  onNavigate(item.page);
                  onClose();
                }}
                className={cn(
                  "group flex h-9 items-center gap-2.5 rounded-md px-2.5 text-[13px] transition-colors",
                  active
                    ? "bg-white/10 font-medium text-white"
                    : "text-slate-400 hover:bg-white/5 hover:text-slate-200",
                )}
                aria-current={active ? "page" : undefined}
              >
                <Icon
                  className={cn(
                    "h-4 w-4 shrink-0",
                    active ? "text-white" : "text-slate-400 group-hover:text-slate-200",
                  )}
                  strokeWidth={1.6}
                />
                <span>{item.label}</span>
                {item.page === "tasks" ? (
                  <span className="ml-auto rounded-full bg-white/10 px-1.5 py-px font-mono text-[12px] tabular-nums text-slate-300">
                    {taskCount}
                  </span>
                ) : null}
                {item.page === "tasks" && bannedCount > 0 ? (
                  <span
                    className="h-1.5 w-1.5 rounded-full bg-red-400"
                    aria-label={`${bannedCount} 个失效任务`}
                  />
                ) : null}
              </button>
            );
          })}

        </nav>

        <div className="mt-auto border-t border-white/10 px-4 py-4">
          <div className="flex items-center gap-2.5">
            <div className="grid h-7 w-7 place-items-center rounded-full bg-slate-700 text-[12px] font-semibold uppercase text-white">
              {username.slice(0, 1) || "Q"}
            </div>
            <div className="min-w-0 flex-1">
              <div className="truncate text-[12px] font-medium text-slate-100">
                {username || "已登录"}
              </div>
              <div className="truncate font-mono text-[12px] text-slate-400">
                管理账号
              </div>
            </div>
            <button
              aria-label="退出登录"
              onClick={onLogout}
              className="grid h-7 w-7 place-items-center rounded-md text-slate-400 hover:bg-white/10 hover:text-white"
            >
              <LogOut className="h-3.5 w-3.5" />
            </button>
          </div>
          <div className="mt-3 font-mono text-[13px] leading-4 tracking-[0.04em] text-slate-400">
            <div>Go server · {version || "—"}</div>
          </div>
        </div>
      </aside>
    </>
  );
}
