const DATE_TIME: Intl.DateTimeFormatOptions = {
  year: "numeric",
  month: "2-digit",
  day: "2-digit",
  hour: "2-digit",
  minute: "2-digit",
  second: "2-digit",
  hourCycle: "h23",
};

const TIME: Intl.DateTimeFormatOptions = {
  hour: "2-digit",
  minute: "2-digit",
  second: "2-digit",
  hourCycle: "h23",
};

function parse(value?: string): Date | null {
  if (!value) return null;
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? null : date;
}

/** Local date and time for API timestamps; empty or malformed input renders as "—". */
export function formatDateTime(value?: string): string {
  const date = parse(value);
  return date ? date.toLocaleString("zh-CN", DATE_TIME) : "—";
}

/** Clock-only variant for the same timestamps. */
export function formatTime(value?: string): string {
  const date = parse(value);
  return date ? date.toLocaleTimeString("zh-CN", TIME) : "—";
}
