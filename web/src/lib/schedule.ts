const WEEKDAYS = ["周日", "周一", "周二", "周三", "周四", "周五", "周六"];

/**
 * 把后端的中文「下次运行」标签（今天 08:00 / 周三 20:00 / —）换算成分钟数，
 * 用于按时间排序而不是按字符串排序。
 */
export function nextRunRank(label: string, now = new Date()): number {
  const trimmed = label.trim();
  if (!trimmed || trimmed === "—") return Number.POSITIVE_INFINITY;
  const [day, time] = trimmed.split(/\s+/, 2);
  const match = /^(\d{1,2}):(\d{2})$/.exec(time ?? "");
  if (!match) return Number.POSITIVE_INFINITY;
  const minutes = Number(match[1]) * 60 + Number(match[2]);
  if (day === "今天") return minutes;
  const index = WEEKDAYS.indexOf(day);
  if (index < 0) return Number.POSITIVE_INFINITY;
  const diff = (index - now.getDay() + 7) % 7;
  return (diff === 0 ? 7 : diff) * 24 * 60 + minutes;
}
