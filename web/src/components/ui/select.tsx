import * as React from "react"
import {
  AnimatePresence,
  motion,
  useReducedMotion,
  type Transition,
  type Variants,
} from "motion/react"
import { CheckIcon, ChevronDownIcon } from "lucide-react"
import { cn } from "cn"

// The trigger and the panel are one surface: the panel inherits the trigger's
// layoutId and grows out of it, so nothing ever detaches.
const MORPH: Transition = { type: "spring", duration: 0.5, bounce: 0.22 }
const ROW = "flex w-full min-w-0 items-center justify-between gap-1.5"
const SIZE_CLASS = "py-2 pr-2 pl-2.5 text-sm"
const HEIGHT_CLASS = {
  default: "h-8",
  sm: "h-7",
} as const
const RADIUS_CLASS = {
  default: "rounded-lg",
  sm: "rounded-[min(var(--radius-md),10px)]",
} as const
const EASE_OUT: [number, number, number, number] = [0.23, 1, 0.32, 1]
const TYPEAHEAD_MS = 500

// Rows blur in one after another, matching the Morph Select surface reveal.
// Long lists switch to a shared reveal so later rows don't trail for seconds.
const LIST_VARIANTS: Variants = {
  hidden: {},
  show: { transition: { staggerChildren: 0.035, delayChildren: 0.08 } },
}
const SHARED_LIST_VARIANTS: Variants = {
  hidden: {},
  show: {},
}
const ITEM_VARIANTS: Variants = {
  hidden: { opacity: 0, y: -6, filter: "blur(3px)" },
  show: {
    opacity: 1,
    y: 0,
    filter: "blur(0px)",
    transition: { duration: 0.18, ease: EASE_OUT },
  },
}

type SelectSize = "sm" | "default"

type ListboxRow = {
  id: string
  label: string
  disabled: boolean
}

type TriggerSurface = {
  className?: string
  height: number
  fontSize: string
  paddingLeft: string
  paddingRight: string
  borderRadius: number
}

const DEFAULT_SURFACE: TriggerSurface = {
  height: 0,
  fontSize: "",
  paddingLeft: "",
  paddingRight: "",
  borderRadius: 10,
}

export type SelectProps = {
  value?: string
  defaultValue?: string
  onValueChange?: (value: string) => void
  items?: readonly { value: string; label: string }[]
  open?: boolean
  defaultOpen?: boolean
  onOpenChange?: (open: boolean) => void
  disabled?: boolean
  className?: string
  children: React.ReactNode
}

type SelectContextValue = {
  value: string | undefined
  open: boolean
  setOpen: (open: boolean) => void
  select: (value: string) => void
  register: (id: string, label: string, disabled: boolean) => void
  unregister: (id: string) => void
  labelFor: (value: string | undefined) => string | undefined
  rows: ListboxRow[]
  close: () => void
  triggerRef: React.RefObject<HTMLButtonElement | null>
  placeholder: string
  setPlaceholder: (placeholder: string) => void
  reduce: boolean
  layoutId: string
  triggerId: string
  listId: string
  headerId: string
  disabled: boolean
  surface: TriggerSurface
  updateSurface: (next: TriggerSurface) => void
}

const SelectContext = React.createContext<SelectContextValue | null>(null)
// The option keyboard focus is on, resolved during render by the content.
const ActiveOptionContext = React.createContext<string | null>(null)
// True inside the hidden copy that only keeps labels registered while closed.
const RegistrarContext = React.createContext(false)

function useSelectContext(component: string) {
  const context = React.useContext(SelectContext)
  if (!context) throw new Error(`${component} must be used within <Select>`)
  return context
}

function Select({
  value: valueProp,
  defaultValue,
  onValueChange,
  items,
  open: openProp,
  defaultOpen = false,
  onOpenChange,
  disabled = false,
  className,
  children,
}: SelectProps) {
  const reduce = useReducedMotion() ?? false
  const baseId = React.useId()
  const rootRef = React.useRef<HTMLDivElement>(null)
  const triggerRef = React.useRef<HTMLButtonElement>(null)
  // Set when the panel closes holding focus: the trigger is unmounted while
  // open, so focus can only go back once it has mounted again.
  const restoreFocus = React.useRef(false)
  const [uncontrolledOpen, setUncontrolledOpen] = React.useState(defaultOpen)
  const [internalValue, setInternalValue] = React.useState(defaultValue)
  const [labels, setLabels] = React.useState<
    Map<string, { label: string; disabled: boolean; count: number }>
  >(new Map())
  const [placeholder, setPlaceholder] = React.useState("")
  const [surface, setSurface] = React.useState<TriggerSurface>(DEFAULT_SURFACE)

  const controlled = valueProp !== undefined
  const value = controlled ? valueProp : internalValue
  const open = openProp ?? uncontrolledOpen

  const setOpen = React.useCallback(
    (next: boolean) => {
      if (openProp === undefined) setUncontrolledOpen(next)
      onOpenChange?.(next)
    },
    [openProp, onOpenChange]
  )

  const close = React.useCallback(() => {
    if (rootRef.current?.contains(document.activeElement)) {
      restoreFocus.current = true
    }
    setOpen(false)
  }, [setOpen])

  const select = React.useCallback(
    (next: string) => {
      if (!controlled) setInternalValue(next)
      onValueChange?.(next)
      close()
    },
    [controlled, onValueChange, close]
  )

  React.useEffect(() => {
    if (open || !restoreFocus.current) return
    restoreFocus.current = false
    triggerRef.current?.focus()
  }, [open])

  // Outside clicks close without pulling focus back to the trigger.
  React.useEffect(() => {
    if (!open) return
    const onPointer = (event: PointerEvent) => {
      if (rootRef.current && !rootRef.current.contains(event.target as Node)) {
        setOpen(false)
      }
    }
    window.addEventListener("pointerdown", onPointer)
    return () => window.removeEventListener("pointerdown", onPointer)
  }, [open, setOpen])

  const register = React.useCallback(
    (id: string, label: string, rowDisabled: boolean) => {
      setLabels((previous) => {
        const entry = previous.get(id)
        const next = new Map(previous)
        next.set(id, {
          label,
          disabled: rowDisabled,
          count: (entry?.count ?? 0) + 1,
        })
        return next
      })
    },
    []
  )

  const unregister = React.useCallback((id: string) => {
    setLabels((previous) => {
      const entry = previous.get(id)
      if (!entry) return previous
      const next = new Map(previous)
      if (entry.count <= 1) next.delete(id)
      else next.set(id, { ...entry, count: entry.count - 1 })
      return next
    })
  }, [])

  const updateSurface = React.useCallback((next: TriggerSurface) => {
    setSurface((previous) =>
      previous.className === next.className &&
      previous.height === next.height &&
      previous.fontSize === next.fontSize &&
      previous.paddingLeft === next.paddingLeft &&
      previous.paddingRight === next.paddingRight &&
      previous.borderRadius === next.borderRadius
        ? previous
        : next
    )
  }, [])

  const rows = React.useMemo(
    () =>
      Array.from(labels, ([id, entry]) => ({
        id,
        label: entry.label,
        disabled: entry.disabled,
      })),
    [labels]
  )

  // `items` wins because rows with rich children register their value as the
  // label; the prop carries the human-readable name.
  const labelFor = React.useCallback(
    (target: string | undefined) => {
      if (target === undefined) return undefined
      return (
        items?.find((item) => item.value === target)?.label ??
        labels.get(target)?.label
      )
    },
    [items, labels]
  )

  const context = React.useMemo<SelectContextValue>(
    () => ({
      value,
      open,
      setOpen,
      select,
      register,
      unregister,
      labelFor,
      rows,
      close,
      triggerRef,
      placeholder,
      setPlaceholder,
      reduce,
      layoutId: `${baseId}-surface`,
      triggerId: `${baseId}-trigger`,
      listId: `${baseId}-list`,
      headerId: `${baseId}-header`,
      disabled,
      surface,
      updateSurface,
    }),
    [
      value,
      open,
      setOpen,
      select,
      register,
      unregister,
      labelFor,
      rows,
      close,
      placeholder,
      reduce,
      baseId,
      disabled,
      surface,
      updateSurface,
    ]
  )

  return (
    <SelectContext.Provider value={context}>
      <div
        ref={rootRef}
        className={cn("relative w-full", className, surface.className)}
      >
        {children}
      </div>
    </SelectContext.Provider>
  )
}

type SelectTriggerProps = {
  className?: string
  size?: SelectSize
  id?: string
  disabled?: boolean
  "aria-label"?: string
  "aria-labelledby"?: string
  "aria-describedby"?: string
  "aria-invalid"?: React.AriaAttributes["aria-invalid"]
  children: React.ReactNode
}

function SelectTrigger({
  className,
  size = "default",
  id,
  children,
  disabled,
  ...props
}: SelectTriggerProps) {
  const context = useSelectContext("SelectTrigger")
  const sizerRef = React.useRef<HTMLDivElement>(null)
  const { updateSurface } = context

  // Measure the closed trigger so the panel header reproduces its row and the
  // morph stays seamless for compact overrides such as `h-7 text-[12px]`.
  React.useLayoutEffect(() => {
    const node = sizerRef.current
    if (!node) return
    const styles = getComputedStyle(node)
    updateSurface({
      className,
      height: node.offsetHeight,
      fontSize: styles.fontSize,
      paddingLeft: styles.paddingLeft,
      paddingRight: styles.paddingRight,
      borderRadius: Number.parseFloat(styles.borderTopLeftRadius) || 10,
    })
  }, [className, size, updateSurface])

  return (
    <>
      <div
        ref={sizerRef}
        aria-hidden
        inert
        className={cn(
          ROW,
          SIZE_CLASS,
          HEIGHT_CLASS[size],
          RADIUS_CLASS[size],
          "invisible border border-input",
          className
        )}
      />

      <AnimatePresence initial={false} mode="popLayout">
        {!context.open ? (
          <motion.button
            key="trigger"
            ref={context.triggerRef}
            layoutId={context.layoutId}
            transition={context.reduce ? { duration: 0 } : MORPH}
            type="button"
            id={id ?? context.triggerId}
            role="combobox"
            aria-haspopup="listbox"
            aria-expanded={false}
            aria-controls={context.listId}
            disabled={disabled ?? context.disabled}
            onClick={() => context.setOpen(true)}
            onKeyDown={(event) => {
              if (event.key === "ArrowDown" || event.key === "ArrowUp") {
                event.preventDefault()
                context.setOpen(true)
              }
            }}
            className={cn(
              ROW,
              SIZE_CLASS,
              HEIGHT_CLASS[size],
              RADIUS_CLASS[size],
              "absolute inset-x-0 top-0 z-10 cursor-pointer border border-input bg-popover text-popover-foreground whitespace-nowrap outline-none select-none transition-colors focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 disabled:cursor-not-allowed disabled:opacity-50 [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4",
              className
            )}
            {...props}
          >
            {children}
            <motion.span
              layout="position"
              aria-hidden
              className="text-muted-foreground"
            >
              <ChevronDownIcon className="size-4" />
            </motion.span>
          </motion.button>
        ) : null}
      </AnimatePresence>
    </>
  )
}

function SelectValue({
  placeholder,
  className,
}: {
  placeholder?: string
  className?: string
}) {
  const context = useSelectContext("SelectValue")
  const { setPlaceholder } = context

  // The panel header renders the placeholder while the trigger is unmounted.
  React.useLayoutEffect(() => {
    if (placeholder) setPlaceholder(placeholder)
  }, [placeholder, setPlaceholder])

  const label = context.labelFor(context.value)

  return (
    <motion.span
      layout="position"
      className={cn(
        "min-w-0 flex-1 truncate text-left",
        label ? undefined : "text-muted-foreground",
        className
      )}
    >
      {label ?? placeholder ?? ""}
    </motion.span>
  )
}

function SelectContent({
  className,
  children,
}: {
  className?: string
  children: React.ReactNode
}) {
  const context = useSelectContext("SelectContent")
  const listRef = React.useRef<HTMLUListElement>(null)
  const { activeId, onKeyDown, selectActive } = useListbox({
    open: context.open,
    rows: context.rows,
    selected: context.value,
    listRef,
    onSelect: context.select,
  })

  return (
    <>
      {/* Always-mounted, hidden: keeps labels registered while closed so the
          trigger can show the selected value before the first open. */}
      <RegistrarContext.Provider value={true}>
        <div hidden>{children}</div>
      </RegistrarContext.Provider>

      <AnimatePresence initial={false} mode="popLayout">
        {context.open ? (
          <motion.div
            key="panel"
            layoutId={context.layoutId}
            transition={context.reduce ? { duration: 0 } : MORPH}
            style={{ borderRadius: context.surface.borderRadius || undefined }}
            onKeyDown={(event) => {
              if (event.key === "Escape" || event.key === "Tab") {
                event.preventDefault()
                event.stopPropagation()
                context.close()
              }
            }}
            className={cn(
              "absolute inset-x-0 top-0 z-30 overflow-hidden border border-input bg-popover text-popover-foreground shadow-lg",
              className
            )}
          >
            <motion.button
              type="button"
              id={context.headerId}
              layout="position"
              aria-haspopup="listbox"
              aria-expanded
              aria-controls={context.listId}
              onClick={context.close}
              style={{
                height: context.surface.height
                  ? context.surface.height - 2
                  : undefined,
                fontSize: context.surface.fontSize || undefined,
                paddingLeft: context.surface.paddingLeft || undefined,
                paddingRight: context.surface.paddingRight || undefined,
              }}
              className={cn(ROW, SIZE_CLASS, "outline-none")}
            >
              <motion.span
                layout="position"
                className={cn(
                  "min-w-0 flex-1 truncate text-left",
                  context.labelFor(context.value)
                    ? "text-popover-foreground"
                    : "text-muted-foreground"
                )}
              >
                {context.labelFor(context.value) ?? context.placeholder}
              </motion.span>
              <motion.span
                layout="position"
                aria-hidden
                animate={{ rotate: 180 }}
                transition={context.reduce ? { duration: 0 } : MORPH}
                className="text-muted-foreground"
              >
                <ChevronDownIcon className="size-4" />
              </motion.span>
            </motion.button>

            <div className="h-px bg-line" />

            <ActiveOptionContext.Provider value={activeId}>
              <motion.ul
                ref={listRef}
                id={context.listId}
                role="listbox"
                aria-labelledby={context.headerId}
                initial={context.reduce ? false : "hidden"}
                animate="show"
                variants={
                  context.reduce
                    ? undefined
                    : context.rows.length > 8
                      ? SHARED_LIST_VARIANTS
                      : LIST_VARIANTS
                }
                onKeyDown={(event) => {
                  if (event.key === "Enter" || event.key === " ") {
                    event.preventDefault()
                    if (activeId) selectActive(activeId)
                    return
                  }
                  onKeyDown(event)
                }}
                className="max-h-[320px] overflow-y-auto overscroll-contain p-1"
              >
                {children}
              </motion.ul>
            </ActiveOptionContext.Provider>
          </motion.div>
        ) : null}
      </AnimatePresence>
    </>
  )
}

function SelectItem({
  value,
  disabled = false,
  className,
  children,
}: {
  value: string
  disabled?: boolean
  className?: string
  children: React.ReactNode
}) {
  const context = useSelectContext("SelectItem")
  const activeId = React.useContext(ActiveOptionContext)
  const registrarOnly = React.useContext(RegistrarContext)
  const selected = context.value === value
  const label = typeof children === "string" ? children : value

  React.useLayoutEffect(() => {
    context.register(value, label, disabled)
    return () => context.unregister(value)
  }, [context.register, context.unregister, value, label, disabled])

  // The hidden copy only registers; no stray options outside the listbox.
  if (registrarOnly) return null

  return (
    <motion.li
      variants={context.reduce ? undefined : ITEM_VARIANTS}
      role="option"
      aria-selected={selected}
      aria-disabled={disabled || undefined}
      tabIndex={activeId === value ? 0 : -1}
      data-listbox-id={value}
      onClick={() => {
        if (!disabled) context.select(value)
      }}
      className={cn(
        "relative flex w-full cursor-default items-center gap-1.5 rounded-md py-1 pr-2 pl-1.5 text-sm outline-hidden select-none transition-colors hover:bg-accent hover:text-accent-foreground focus:bg-accent focus:text-accent-foreground aria-disabled:pointer-events-none aria-disabled:opacity-50",
        className
      )}
    >
      <div className="min-w-0 flex-1">{children}</div>
      {selected ? <CheckIcon className="size-4 shrink-0" /> : null}
    </motion.li>
  )
}

/**
 * Keyboard model for the open list. Focus follows the active option; arrow
 * keys, Home/End and typeahead move it, and the selected row opens first.
 */
function useListbox({
  open,
  rows,
  selected,
  listRef,
  onSelect,
}: {
  open: boolean
  rows: readonly ListboxRow[]
  selected: string | undefined
  listRef: React.RefObject<HTMLUListElement | null>
  onSelect: (value: string) => void
}) {
  const enabled = React.useMemo(
    () => rows.filter((row) => !row.disabled),
    [rows]
  )
  const [activeId, setActiveId] = React.useState<string | null>(null)
  const [wasOpen, setWasOpen] = React.useState(open)
  const typed = React.useRef({ text: "", at: 0 })

  if (open !== wasOpen) {
    setWasOpen(open)
    if (open) {
      const initial =
        selected && enabled.some((row) => row.id === selected)
          ? selected
          : enabled[0]?.id ?? null
      setActiveId(initial)
    }
  }

  if (activeId !== null && !enabled.some((row) => row.id === activeId)) {
    setActiveId(enabled[0]?.id ?? null)
  }

  React.useEffect(() => {
    if (!open || activeId === null) return
    const option = listRef.current?.querySelector<HTMLElement>(
      `[data-listbox-id="${CSS.escape(activeId)}"]`
    )
    option?.focus({ preventScroll: false })
  }, [open, activeId, listRef])

  const indexOfActive = enabled.findIndex((row) => row.id === activeId)

  const move = (direction: 1 | -1) => {
    if (enabled.length === 0) return
    const from = indexOfActive < 0 ? 0 : indexOfActive
    const next = Math.min(Math.max(from + direction, 0), enabled.length - 1)
    setActiveId(enabled[next].id)
  }

  const onKeyDown = (event: React.KeyboardEvent) => {
    const { key } = event
    if (key === "ArrowDown" || key === "ArrowUp") {
      event.preventDefault()
      move(key === "ArrowDown" ? 1 : -1)
      return
    }
    if (key === "Home" || key === "End") {
      event.preventDefault()
      const row = key === "Home" ? enabled[0] : enabled[enabled.length - 1]
      if (row) setActiveId(row.id)
      return
    }
    if (
      key.length === 1 &&
      key !== " " &&
      !event.ctrlKey &&
      !event.metaKey &&
      !event.altKey
    ) {
      const now = Date.now()
      const char = key.toLowerCase()
      const text =
        now - typed.current.at < TYPEAHEAD_MS
          ? typed.current.text + char
          : char
      typed.current = { text, at: now }
      const from = text.length === 1 ? indexOfActive + 1 : indexOfActive
      for (let step = 0; step < enabled.length; step++) {
        const row = enabled[(from + step + enabled.length) % enabled.length]
        if (row.label.toLowerCase().startsWith(text)) {
          setActiveId(row.id)
          return
        }
      }
    }
  }

  const selectActive = (id: string) => {
    onSelect(id)
  }

  return { activeId, onKeyDown, selectActive }
}

export { Select, SelectContent, SelectItem, SelectTrigger, SelectValue }
