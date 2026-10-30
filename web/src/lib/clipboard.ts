/** Insecure origins expose no Clipboard API, so copy affordances disable up front. */
export function canCopyText(): boolean {
  return typeof navigator !== "undefined" && Boolean(navigator.clipboard?.writeText);
}
