import { LogOut, X } from "lucide-react";
import type { Page } from "../../lib/types";
import { cn } from "../../lib/utils";
import { BrandMark } from "../BrandMark";
import { MenuIcon } from "./menu-icons";
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

const workspaceItems: { page: Page; label: string }[] = [
  { page: "overview", label: "总览" },
  { page: "tasks", label: "任务" },
  { page: "files", label: "文件" },
  { page: "runs", label: "运行记录" },
];

const systemItems: { page: Page; label: string }[] = [
  { page: "settings", label: "设置" },
];

type AppSidebarProps = {
  page: Page;
  bannedCount: number;
  username: string;
  onNavigate: (page: Page) => void;
  onLogout: () => void;
  onSidebarResize: (width: number) => void;
};

export function AppSidebar({
  page,
  bannedCount,
  username,
  onNavigate,
  onLogout,
  onSidebarResize,
}: AppSidebarProps) {
  const { setOpenMobile } = useSidebar();

  const handleNavigate = (next: Page) => {
    onNavigate(next);
    setOpenMobile(false);
  };

  const renderItem = (item: (typeof workspaceItems)[number]) => {
    const active = page === item.page;
    return (
      <SidebarMenuItem key={item.page}>
        <SidebarMenuButton
          onClick={() => handleNavigate(item.page)}
          aria-current={active ? "page" : undefined}
          // 激活态在渲染期定死（hover/press 不改样式，白胶囊保持稳定），
          // hover 只挂在非激活项上。
          className={cn(
            "h-9 gap-2.5 rounded-[10px] px-2.5 text-[13px] transition-[background-color,color,box-shadow] duration-150 ease-out",
            active
              ? "bg-panel font-medium text-ink shadow-[0_1px_3px_rgb(16_24_40/0.10)] hover:bg-panel active:bg-panel dark:bg-white/[0.08] dark:shadow-none dark:hover:bg-white/[0.08] dark:active:bg-white/[0.08]"
              : "text-muted-ink hover:bg-sidebar-accent hover:text-ink",
          )}
        >
          <MenuIcon
            id={item.page as Parameters<typeof MenuIcon>[0]["id"]}
            active={active}
            className={cn(
              "h-4 w-4 shrink-0 transition-colors duration-150 ease-out",
              active ? "text-brand" : "text-subtle",
            )}
          />
          <span>{item.label}</span>
        </SidebarMenuButton>
        {item.page === "tasks" && bannedCount > 0 ? (
          <SidebarMenuBadge
            title={`${bannedCount} 个失效任务需要处理`}
            aria-label={`${bannedCount} 个失效任务需要处理`}
            className={cn(
              "top-2.5 h-4 min-w-4 rounded-full px-1.5 font-mono text-[11px] font-normal",
              "bg-red-500/10 text-red-600 dark:bg-red-500/25 dark:text-red-300",
            )}
          >
            {bannedCount}
          </SidebarMenuBadge>
        ) : null}
      </SidebarMenuItem>
    );
  };

  return (
    <Sidebar collapsible="offcanvas">
      <SidebarHeader className="gap-0 px-3.5 pb-1.5 pt-4">
        <div className="flex items-center gap-2.5">
          <BrandMark className="h-8 w-8" />
          <div className="min-w-0 flex-1">
            <div className="truncate text-[14px] font-semibold tracking-wide text-ink">
              夸克自动转存
            </div>
          </div>
          <button
            aria-label="关闭导航"
            onClick={() => setOpenMobile(false)}
            className="grid h-7 w-7 shrink-0 place-items-center rounded-md text-subtle hover:bg-sidebar-accent hover:text-ink md:hidden"
          >
            <X className="h-4 w-4" />
          </button>
        </div>
      </SidebarHeader>

      <SidebarContent className="pt-3 pb-3">
        {/* pr-3.5：菜单右缘与 Rail 拖拽线之间留出间距，并与 header 的 px-3.5 对齐 */}
        <SidebarGroup className="pr-3.5">
          <SidebarGroupLabel className="h-7 px-2.5 text-[11px] font-medium tracking-[0.12em] text-subtle/80">
            工作台
          </SidebarGroupLabel>
          <SidebarGroupContent>
            <SidebarMenu className="gap-1">
              {workspaceItems.map(renderItem)}
            </SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>

        <SidebarGroup className="mt-1 pr-3.5">
          <SidebarGroupLabel className="h-7 px-2.5 text-[11px] font-medium tracking-[0.12em] text-subtle/80">
            系统
          </SidebarGroupLabel>
          <SidebarGroupContent>
            <SidebarMenu className="gap-1">
              {systemItems.map(renderItem)}
            </SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>
      </SidebarContent>

      <SidebarFooter className="gap-0 border-t border-sidebar-border px-3.5 pb-3.5 pt-3">
        <div className="flex items-center gap-2.5">
          <div className="grid h-7 w-7 shrink-0 place-items-center rounded-full bg-brand-soft text-[12px] font-semibold uppercase text-brand-ink">
            {username.slice(0, 1) || "Q"}
          </div>
          <div className="min-w-0 flex-1">
            <div className="truncate text-[12px] font-medium text-ink">
              {username || "已登录"}
            </div>
            <div className="truncate text-[11px] text-subtle">管理账号</div>
          </div>
          <button
            aria-label="退出登录"
            onClick={onLogout}
            className="grid h-7 w-7 shrink-0 place-items-center rounded-md text-subtle hover:bg-sidebar-accent hover:text-ink"
          >
            <LogOut className="h-3.5 w-3.5" />
          </button>
        </div>
      </SidebarFooter>

      <SidebarRail onResize={onSidebarResize} />
    </Sidebar>
  );
}
