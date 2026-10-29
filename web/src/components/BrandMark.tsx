import { cn } from "../lib/utils";

export function BrandMark({ className }: { className?: string }) {
  return (
    <img
      src="/brand.svg"
      alt="QAS"
      className={cn("shrink-0", className)}
      draggable={false}
    />
  );
}
