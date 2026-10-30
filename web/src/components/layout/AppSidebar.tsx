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
import { BrandMark } from "../BrandMark";
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuBadge,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarRail,
  useSidebar,
} from "../ui/sidebar";

const workspaceItems: { page: Page; label: string; icon: typeof LayoutDashboard }[] = [
  { page: "overview", label: "总览", icon: LayoutDashboard },
  { page: "tasks", label: "任务", icon: ListVideo },
  { page: "files", label: "文件", icon: FolderOpen },
  { page: "runs", label: "运行记录", icon: ScrollText },
];

const systemItems: { page: Page; label: string; icon: typeof LayoutDashboard }[] = [
  { page: "settings", label: "设置", icon: Settings2 },
];

type AppSidebarProps = {
  page: Page;
  taskCount: number;
  bannedCount: number;
  username: string;
  version: string;
  onNavigate: (page: Page) => void;
  onLogout: () => void;
};

export function AppSidebar({
  page,
  taskCount,
  bannedCount,
  username,
  version,
  onNavigate,
  onLogout,
}: AppSidebarProps) {
  const { setOpenMobile } = useSidebar();

  const handleNavigate = (next: Page) => {
    onNavigate(next);
    setOpenMobile(false);
  };

  const renderItem = (item: (typeof workspaceItems)[number]) => {
    const Icon = item.icon;
    const active = page === item.page;
    return (
      <SidebarMenuItem key={item.page}>
        {active ? (
          <span
            aria-hidden
            className="pointer-events-none absolute top-1/2 left-1 h-4 w-0.5 -translate-y-1/2 rounded-full bg-brand"
          />
        ) : null}
        <SidebarMenuButton
          isActive={active}
          onClick={() => handleNavigate(item.page)}
          aria-current={active ? "page" : undefined}
          className="h-9 gap-2.5 px-2.5 text-[13px] text-sidebar-foreground/65 transition-[background-color,color] duration-150 ease-out hover:text-sidebar-accent-foreground data-active:bg-white/[0.11] data-active:text-sidebar-accent-foreground"
        >
          <Icon
            className={cn(
              "h-4 w-4 shrink-0 transition-colors duration-150 ease-out",
              active ? "text-white" : "text-sidebar-foreground/50",
            )}
            strokeWidth={1.6}
          />
          <span>{item.label}</span>
        </SidebarMenuButton>
        {item.page === "tasks" ? (
          <SidebarMenuBadge
            title={bannedCount > 0 ? `${bannedCount} 个失效任务` : undefined}
            className={cn(
              "top-2.5 h-4 min-w-4 rounded-full px-1.5 font-mono text-[11px] font-normal",
              bannedCount > 0
                ? "bg-red-500/20 text-red-300"
                : "bg-white/8 text-sidebar-foreground/60",
            )}
          >
            {taskCount}
          </SidebarMenuBadge>
        ) : null}
      </SidebarMenuItem>
    );
  };

  return (
    <Sidebar collapsible="offcanvas" className="border-sidebar-border">
      <SidebarHeader className="gap-0 px-3.5 pb-1.5 pt-4">
        <div className="flex items-center gap-2.5">
          <BrandMark className="h-8 w-8" />
          <div className="min-w-0 flex-1">
            <div className="truncate text-[14px] font-semibold tracking-wide text-sidebar-accent-foreground">
              夸克自动转存
            </div>
            <div className="truncate font-mono text-[11px] tracking-[0.16em] text-sidebar-foreground/40">
              QAS · SELF-HOSTED
            </div>
          </div>
          <button
            aria-label="关闭导航"
            onClick={() => setOpenMobile(false)}
            className="grid h-7 w-7 shrink-0 place-items-center rounded-md text-sidebar-foreground/50 hover:bg-sidebar-accent hover:text-white md:hidden"
          >
            <X className="h-4 w-4" />
          </button>
        </div>
      </SidebarHeader>

      <SidebarContent className="pt-3">
        <SidebarGroup>
          <SidebarGroupLabel className="h-7 px-2.5 text-[11px] font-medium tracking-[0.12em] text-sidebar-foreground/35">
            工作台
          </SidebarGroupLabel>
          <SidebarGroupContent>
            <SidebarMenu>{workspaceItems.map(renderItem)}</SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>

        <SidebarGroup className="mt-1">
          <SidebarGroupLabel className="h-7 px-2.5 text-[11px] font-medium tracking-[0.12em] text-sidebar-foreground/35">
            系统
          </SidebarGroupLabel>
          <SidebarGroupContent>
            <SidebarMenu>{systemItems.map(renderItem)}</SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>
      </SidebarContent>

      <SidebarFooter className="gap-0 border-t border-sidebar-border px-3.5 pb-3.5 pt-3">
        <div className="flex items-center gap-2.5">
          <div className="grid h-7 w-7 shrink-0 place-items-center rounded-full bg-sidebar-accent text-[12px] font-semibold uppercase text-white">
            {username.slice(0, 1) || "Q"}
          </div>
          <div className="min-w-0 flex-1">
            <div className="truncate text-[12px] font-medium text-sidebar-accent-foreground">
              {username || "已登录"}
            </div>
            <div className="truncate text-[11px] text-sidebar-foreground/40">
              管理账号
            </div>
          </div>
          <button
            aria-label="退出登录"
            onClick={onLogout}
            className="grid h-7 w-7 shrink-0 place-items-center rounded-md text-sidebar-foreground/50 hover:bg-sidebar-accent hover:text-white"
          >
            <LogOut className="h-3.5 w-3.5" />
          </button>
        </div>
        <div className="mt-2.5 flex items-center gap-1.5 font-mono text-[11px] leading-4 tracking-[0.04em] text-sidebar-foreground/35">
          <span>Go server</span>
          <span aria-hidden>·</span>
          <span>{version || "—"}</span>
        </div>
      </SidebarFooter>

      <SidebarRail />
    </Sidebar>
  );
}
