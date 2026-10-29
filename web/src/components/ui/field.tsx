import * as React from "react";
import { cn } from "../../lib/utils";
import { Switch } from "./switch";

export function Label({
  className,
  ...props
}: React.LabelHTMLAttributes<HTMLLabelElement>) {
  return (
    <label
      className={cn("text-[12px] font-medium text-ink-soft", className)}
      {...props}
    />
  );
}

type FieldProps = {
  label: React.ReactNode;
  hint?: React.ReactNode;
  htmlFor?: string;
  className?: string;
  children: React.ReactNode;
};

export function Field({ label, hint, htmlFor, className, children }: FieldProps) {
  const autoId = React.useId();
  // Grouped children (e.g. input + button) cannot take a label's `for`;
  // label the wrapper instead so the markup stays valid.
  const grouped =
    React.isValidElement(children) &&
    typeof children.type === "string" &&
    ["div", "span"].includes(children.type);
  if (grouped) {
    return (
      <div className={cn("space-y-1.5", className)}>
        <div role="group" aria-labelledby={`${autoId}-label`}>
          <span
            id={`${autoId}-label`}
            className="mb-1.5 block text-[12px] font-medium text-ink-soft"
          >
            {label}
          </span>
          {children}
        </div>
        {hint ? <p className="text-[12px] leading-4 text-muted">{hint}</p> : null}
      </div>
    );
  }
  const id = htmlFor ?? autoId;
  const control = React.isValidElement<{ id?: string }>(children)
    ? React.cloneElement(children, { id: children.props.id ?? id })
    : children;
  return (
    <div className={cn("space-y-1.5", className)}>
      <Label htmlFor={id}>{label}</Label>
      {control}
      {hint ? <p className="text-[12px] leading-4 text-muted">{hint}</p> : null}
    </div>
  );
}

export function SectionHeading({
  title,
  description,
  action,
  className,
}: {
  title: React.ReactNode;
  description?: React.ReactNode;
  action?: React.ReactNode;
  className?: string;
}) {
  return (
    <div className={cn("flex items-start justify-between gap-4", className)}>
      <div className="min-w-0">
        <h2 className="text-[14px] font-semibold text-ink">{title}</h2>
        {description ? (
          <p className="mt-1 max-w-[70ch] text-[12px] leading-5 text-muted">
            {description}
          </p>
        ) : null}
      </div>
      {action ? <div className="shrink-0">{action}</div> : null}
    </div>
  );
}

export function SwitchRow({
  label,
  description,
  checked,
  onCheckedChange,
  disabled,
}: {
  label: React.ReactNode;
  description?: React.ReactNode;
  checked: boolean;
  onCheckedChange: (checked: boolean) => void;
  disabled?: boolean;
}) {
  return (
    <div className="flex items-start justify-between gap-4 py-2">
      <div className="min-w-0">
        <div className="text-[13px] font-medium text-ink">{label}</div>
        {description ? (
          <div className="mt-0.5 text-[12px] leading-4 text-muted">
            {description}
          </div>
        ) : null}
      </div>
      <Switch
        checked={checked}
        onCheckedChange={onCheckedChange}
        disabled={disabled}
        aria-label={typeof label === "string" ? label : undefined}
      />
    </div>
  );
}

export function Divider({ className }: { className?: string }) {
  return <div className={cn("h-px bg-line", className)} />;
}

export function Mono({
  children,
  className,
}: {
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <span className={cn("font-mono text-[12px] tabular-nums", className)}>
      {children}
    </span>
  );
}
