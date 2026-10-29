import { useState } from "react";
import { format, isValid, parseISO } from "date-fns";
import { CalendarDays } from "lucide-react";
import { zhCN } from "react-day-picker/locale";
import { cn } from "../../lib/utils";
import { Button } from "./button";
import { Calendar } from "./calendar";
import { Popover, PopoverContent, PopoverTrigger } from "./popover";

type DatePickerProps = {
  value?: string;
  onChange: (value: string) => void;
  placeholder?: string;
  id?: string;
  name?: string;
  className?: string;
  "aria-label"?: string;
};

function parseValue(value?: string) {
  if (!value) return undefined;
  const parsed = parseISO(value);
  return isValid(parsed) ? parsed : undefined;
}

export function DatePicker({
  value,
  onChange,
  placeholder = "选择日期",
  id,
  name,
  className,
  "aria-label": ariaLabel,
}: DatePickerProps) {
  const [open, setOpen] = useState(false);
  const selected = parseValue(value);
  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger
        render={
          <button
            type="button"
            id={id}
            name={name}
            aria-label={ariaLabel}
            className={cn(
              "flex h-8 w-full items-center justify-between gap-2 rounded-md border border-line bg-panel px-2 text-[13px] text-ink transition-colors focus-visible:border-brand focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand/15",
              !selected && "text-subtle",
              className,
            )}
          >
            <span>{selected ? format(selected, "yyyy-MM-dd") : placeholder}</span>
            <CalendarDays
              className="h-3.5 w-3.5 shrink-0 text-subtle"
              strokeWidth={1.7}
            />
          </button>
        }
      />
      <PopoverContent className="w-auto p-1">
        <Calendar
          mode="single"
          selected={selected}
          defaultMonth={selected}
          locale={zhCN}
          weekStartsOn={1}
          onSelect={(date) => {
            onChange(date ? format(date, "yyyy-MM-dd") : "");
            setOpen(false);
          }}
        />
        <div className="mt-1 flex items-center justify-between border-t border-line-soft px-1 pt-1.5">
          <Button
            type="button"
            variant="ghost"
            size="sm"
            onClick={() => {
              onChange("");
              setOpen(false);
            }}
          >
            清空
          </Button>
          <Button
            type="button"
            variant="ghost"
            size="sm"
            onClick={() => {
              onChange(format(new Date(), "yyyy-MM-dd"));
              setOpen(false);
            }}
          >
            今天
          </Button>
        </div>
      </PopoverContent>
    </Popover>
  );
}
