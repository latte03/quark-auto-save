import * as React from "react";
import { cn } from "../../lib/utils";

export const Input = React.forwardRef<
  HTMLInputElement,
  React.InputHTMLAttributes<HTMLInputElement>
>(({ className, type = "text", id, ...props }, ref) => {
  const autoId = React.useId();
  return (
  <input
    ref={ref}
    id={id ?? autoId}
    type={type}
    className={cn(
      "h-8 w-full rounded-md border border-line bg-panel px-2.5 text-[13px] text-ink shadow-[0_1px_1px_rgba(16,24,40,0.02)] transition-colors placeholder:text-subtle focus-visible:border-brand focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand/15 disabled:cursor-not-allowed disabled:bg-line-soft disabled:text-muted",
      className,
    )}
    {...props}
  />
  );
});
Input.displayName = "Input";

export const Textarea = React.forwardRef<
  HTMLTextAreaElement,
  React.TextareaHTMLAttributes<HTMLTextAreaElement>
>(({ className, id, ...props }, ref) => {
  const autoId = React.useId();
  return (
    <textarea
      ref={ref}
      id={id ?? autoId}
      className={cn(
        "min-h-20 w-full resize-y rounded-md border border-line bg-panel px-2.5 py-2 text-[13px] leading-5 text-ink transition-colors placeholder:text-subtle focus-visible:border-brand focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand/15",
        className,
      )}
      {...props}
    />
  );
});
Textarea.displayName = "Textarea";


export const Checkbox = React.forwardRef<
  HTMLInputElement,
  React.InputHTMLAttributes<HTMLInputElement>
>(({ className, ...props }, ref) => (
  <input
    ref={ref}
    type="checkbox"
    className={cn(
      "h-3.5 w-3.5 rounded border-line text-brand accent-brand focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand/25",
      className,
    )}
    {...props}
  />
));
Checkbox.displayName = "Checkbox";
