/** 统一目录路径：去空白、补前导斜杠、折叠重复斜杠、去尾斜杠。空输入返回空串。 */
export function normalizeDirPath(raw: string): string {
  const trimmed = raw.trim();
  if (!trimmed) return "";
  if (trimmed === "/") return "/";
  const absolute = trimmed.startsWith("/") ? trimmed : `/${trimmed}`;
  return absolute.replace(/\/{2,}/g, "/").replace(/(.)\/$/, "$1");
}

export function joinDirPath(base: string, name: string): string {
  return base === "/" ? `/${name}` : `${base}/${name}`;
}
