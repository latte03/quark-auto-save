import * as React from "react"
import { Tabs as TabsPrimitive } from "@base-ui/react/tabs"
import { cva, type VariantProps } from "class-variance-authority"
import { motion, useReducedMotion } from "motion/react"
import { SPRING_LAYOUT, SPRING_SWAP } from "@/lib/ease"
import { cn } from "@/lib/utils"

/** Space between an open selected icon and its label, in pixels. */
const ICON_GAP = 6

type TabsContextValue = {
  value: TabsPrimitive.Tab.Value
  reduce: boolean
}

const TabsContext = React.createContext<TabsContextValue | null>(null)

function useTabsContext() {
  const context = React.useContext(TabsContext)
  if (!context) throw new Error("Tabs.* must be used inside <Tabs>")
  return context
}

function Tabs({
  className,
  orientation = "horizontal",
  value,
  defaultValue,
  onValueChange,
  ...props
}: TabsPrimitive.Root.Props) {
  const [uncontrolled, setUncontrolled] = React.useState<TabsPrimitive.Tab.Value>(
    () => defaultValue ?? 0
  )
  const controlled = value !== undefined
  const current = controlled ? value : uncontrolled
  const reduce = Boolean(useReducedMotion())

  const handleValueChange = React.useCallback(
    (
      next: TabsPrimitive.Tab.Value,
      details: TabsPrimitive.Root.ChangeEventDetails
    ) => {
      if (!controlled) setUncontrolled(next)
      onValueChange?.(next, details)
    },
    [controlled, onValueChange]
  )

  return (
    <TabsContext.Provider value={{ value: current, reduce }}>
      <TabsPrimitive.Root
        data-slot="tabs"
        orientation={orientation}
        data-orientation={orientation}
        value={current}
        onValueChange={handleValueChange}
        className={cn(
          "group/tabs flex gap-2 data-horizontal:flex-col",
          className
        )}
        {...props}
      />
    </TabsContext.Provider>
  )
}

const tabsListVariants = cva(
  "group/tabs-list relative isolate inline-flex w-fit items-center justify-center rounded-lg p-[3px] text-muted-foreground group-data-horizontal/tabs:h-8 group-data-vertical/tabs:h-fit group-data-vertical/tabs:flex-col data-[variant=line]:rounded-none",
  {
    variants: {
      variant: {
        default: "bg-muted",
        line: "gap-1 bg-transparent",
      },
    },
    defaultVariants: {
      variant: "default",
    },
  }
)

type Box = { x: number; y: number; width: number; height: number }

function sameBox(a: Box, b: Box) {
  return (
    Math.abs(a.x - b.x) < 0.5 &&
    Math.abs(a.y - b.y) < 0.5 &&
    Math.abs(a.width - b.width) < 0.5 &&
    Math.abs(a.height - b.height) < 0.5
  )
}

function TabsList({
  className,
  variant = "default",
  children,
  ...props
}: TabsPrimitive.List.Props & VariantProps<typeof tabsListVariants>) {
  const { value } = useTabsContext()
  const reduce = useReducedMotion()
  const listRef = React.useRef<HTMLDivElement>(null)
  const [box, setBox] = React.useState<Box | null>(null)
  const tracked = variant !== "line"

  React.useLayoutEffect(() => {
    const list = listRef.current
    if (!list || !tracked) return

    const measure = () => {
      const target = Array.from(
        list.querySelectorAll<HTMLElement>('[role="tab"]')
      ).find((tab) => tab.getAttribute("aria-selected") === "true")
      if (!target) {
        setBox(null)
        return
      }
      // Measure the settled layout: open the selected icon and fold the rest
      // for one synchronous read, then hand the styles back before paint. A
      // read taken mid-spring would leave the pill chasing the tab's width.
      const icons = Array.from(
        list.querySelectorAll<HTMLElement>("[data-tabs-icon]")
      )
      const saved = icons.map(
        (icon) => [icon.style.width, icon.style.marginRight] as const
      )
      for (const icon of icons) {
        const open = icon.closest('[role="tab"]') === target
        icon.style.width = open ? "auto" : "0px"
        icon.style.marginRight = open ? `${ICON_GAP}px` : "0px"
      }
      const next = {
        x: target.offsetLeft,
        y: target.offsetTop,
        width: target.getBoundingClientRect().width,
        height: target.offsetHeight,
      }
      icons.forEach((icon, index) => {
        icon.style.width = saved[index][0]
        icon.style.marginRight = saved[index][1]
      })
      if (next.width <= 0 || next.height <= 0) return
      setBox((previous) =>
        previous && sameBox(previous, next) ? previous : next
      )
    }

    measure()
    const observer = new ResizeObserver(measure)
    observer.observe(list)
    for (const tab of list.querySelectorAll('[role="tab"]')) {
      observer.observe(tab)
    }
    return () => observer.disconnect()
  }, [tracked, value])

  return (
    <TabsPrimitive.List
      ref={listRef}
      data-slot="tabs-list"
      data-variant={variant}
      className={cn(tabsListVariants({ variant }), className)}
      {...props}
    >
      {tracked && box ? (
        <motion.span
          aria-hidden="true"
          data-slot="tabs-indicator"
          initial={false}
          animate={box}
          transition={reduce ? { duration: 0 } : SPRING_LAYOUT}
          className="pointer-events-none absolute left-0 top-0 -z-10 rounded-md border border-transparent bg-background shadow-sm dark:border-input dark:bg-input/30"
        />
      ) : null}
      {children}
    </TabsPrimitive.List>
  )
}

type TabsTriggerProps = TabsPrimitive.Tab.Props & {
  /** Shown before the label only while this tab is selected. */
  icon?: React.ReactNode
}

function TabsTrigger({
  className,
  icon,
  children,
  ...props
}: TabsTriggerProps) {
  const { value, reduce } = useTabsContext()
  const active = value === props.value

  return (
    <TabsPrimitive.Tab
      data-slot="tabs-trigger"
      className={cn(
        "relative inline-flex h-[calc(100%-1px)] flex-1 items-center justify-center rounded-md border border-transparent px-1.5 py-0.5 text-sm font-medium whitespace-nowrap text-foreground/60 transition-[color,opacity] duration-150 ease-out group-data-vertical/tabs:w-full group-data-vertical/tabs:justify-start hover:text-foreground focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 focus-visible:outline-1 focus-visible:outline-ring disabled:pointer-events-none disabled:opacity-50 has-data-[icon=inline-end]:pr-1 has-data-[icon=inline-start]:pl-1 aria-disabled:pointer-events-none aria-disabled:opacity-50 dark:text-muted-foreground dark:hover:text-foreground data-active:text-foreground dark:data-active:text-foreground [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-3",
        "after:absolute after:bg-foreground after:opacity-0 after:transition-opacity group-data-horizontal/tabs:after:inset-x-0 group-data-horizontal/tabs:after:bottom-[-5px] group-data-horizontal/tabs:after:h-0.5 group-data-vertical/tabs:after:inset-y-0 group-data-vertical/tabs:after:-right-1 group-data-vertical/tabs:after:w-0.5 group-data-[variant=line]/tabs-list:data-active:after:opacity-100",
        className
      )}
      {...props}
    >
      {icon ? (
        <motion.span
          aria-hidden="true"
          data-tabs-icon=""
          initial={false}
          animate={{
            width: active ? "auto" : 0,
            marginRight: active ? ICON_GAP : 0,
            opacity: active ? 1 : 0,
            scale: active ? 1 : 0.6,
          }}
          transition={reduce ? { duration: 0 } : SPRING_SWAP}
          className="inline-flex shrink-0 items-center justify-center overflow-hidden"
        >
          {icon}
        </motion.span>
      ) : null}
      {children}
    </TabsPrimitive.Tab>
  )
}

function TabsContent({ className, ...props }: TabsPrimitive.Panel.Props) {
  return (
    <TabsPrimitive.Panel
      data-slot="tabs-content"
      className={cn(
        "flex-1 rounded-sm text-sm outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50",
        className
      )}
      {...props}
    />
  )
}

export { Tabs, TabsList, TabsTrigger, TabsContent, tabsListVariants }
