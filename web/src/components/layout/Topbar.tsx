import { Play, Plus, ScrollText, Search } from "lucide-react";
import { Button } from "../ui/button";
import { Input } from "../ui/input";
import { SidebarTrigger } from "../ui/sidebar";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "../ui/tooltip";

type TopbarProps = {
  /** Only the task page owns the search field; other pages leave the slot empty. */
  search: string | null;
  onSearch: (value: string) => void;
  onRunAll: () => void;
  onAddTask: () => void;
  onOpenLog: () => void;
};

export function Topbar({
  search,
  onSearch,
  onRunAll,
  onAddTask,
  onOpenLog,
}: TopbarProps) {
  return (
    <header className="sticky top-0 z-20 flex h-14 shrink-0 items-center gap-2 border-b border-line bg-panel/90 px-4 backdrop-blur-md supports-backdrop-filter:bg-panel/80">
      <SidebarTrigger
        aria-label="切换侧边栏"
        title="切换侧边栏"
        className="-ml-1.5 text-muted-ink"
      />
      <div className="ml-auto flex items-center gap-2">
        {search !== null ? (
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
        ) : null}
        <Tooltip>
          <TooltipTrigger
            render={
              <Button
                variant="ghost"
                size="icon"
                aria-label="打开运行记录"
                onClick={onOpenLog}
              >
                <ScrollText className="h-3.5 w-3.5" />
              </Button>
            }
          />
          <TooltipContent>运行记录</TooltipContent>
        </Tooltip>
        <span className="mx-0.5 h-5 w-px shrink-0 bg-line" aria-hidden />
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
