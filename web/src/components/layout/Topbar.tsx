import { Menu, Play, Plus, ScrollText, Search } from "lucide-react";
import type { Page } from "../../lib/types";
import { Button } from "../ui/button";
import { Input } from "../ui/input";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "../ui/tooltip";

const pageLabels: Record<Page, string> = {
  overview: "总览",
  tasks: "任务",
  files: "文件",
  runs: "运行记录",
  settings: "设置",
};

type TopbarProps = {
  page: Page;
  search: string;
  onSearch: (value: string) => void;
  onMenu: () => void;
  onRunAll: () => void;
  onAddTask: () => void;
  onOpenLog: () => void;
};

export function Topbar({
  page,
  search,
  onSearch,
  onMenu,
  onRunAll,
  onAddTask,
  onOpenLog,
}: TopbarProps) {
  return (
    <header className="sticky top-0 z-20 flex h-14 shrink-0 items-center gap-3 border-b border-line bg-panel/95 px-4 backdrop-blur-sm lg:h-[60px] lg:px-7">
      <button
        aria-label="打开导航"
        onClick={onMenu}
        className="grid h-8 w-8 place-items-center rounded-md text-muted hover:bg-line-soft hover:text-ink lg:hidden"
      >
        <Menu className="h-4 w-4" />
      </button>
      <div className="flex min-w-0 items-baseline gap-2">
        <h1 className="truncate text-[17px] font-semibold tracking-[-0.01em] text-ink">
          {pageLabels[page]}
        </h1>
        <span className="hidden font-mono text-[12px] tracking-[0.08em] text-subtle md:inline">
          QAS / {pageLabels[page].toUpperCase()}
        </span>
      </div>

      <div className="ml-auto flex items-center gap-2">
        <div className="relative hidden md:block">
          <Search className="pointer-events-none absolute left-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-subtle" />
          <Input
            id="qas-search"
            name="qas-search"
            type="search"
            autoComplete="off"
            value={search}
            onChange={(event) => onSearch(event.target.value)}
            aria-label="搜索任务或路径"
            placeholder="搜索任务或路径"
            className="h-8 w-[220px] pl-8 text-[12px]"
          />
        </div>
        <Tooltip>
          <TooltipTrigger asChild>
            <Button
              variant="ghost"
              size="icon"
              aria-label="打开运行记录"
              onClick={onOpenLog}
            >
              <ScrollText className="h-3.5 w-3.5" />
            </Button>
          </TooltipTrigger>
          <TooltipContent>运行记录</TooltipContent>
        </Tooltip>
        <Button variant="secondary" size="sm" onClick={onRunAll}>
          <Play className="h-3.5 w-3.5" />
          <span className="hidden sm:inline">运行全部</span>
        </Button>
        <Button size="sm" onClick={onAddTask}>
          <Plus className="h-3.5 w-3.5" />
          <span className="hidden sm:inline">新增任务</span>
          <span className="sm:hidden">新增</span>
        </Button>
      </div>
    </header>
  );
}
