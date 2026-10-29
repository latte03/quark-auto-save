import { useId } from "react";
import { cn } from "../lib/utils";
import { Field, FieldDescription, FieldLabel } from "./ui/field";
import { Switch } from "./ui/switch";

export function SwitchField({
  label,
  description,
  checked,
  onCheckedChange,
  disabled,
  className,
}: {
  label: React.ReactNode;
  description?: React.ReactNode;
  checked: boolean;
  onCheckedChange: (checked: boolean) => void;
  disabled?: boolean;
  className?: string;
}) {
  const id = useId();
  return (
    <Field className={cn("gap-1 py-1.5", className)}>
      <div className="flex min-w-0 items-center justify-between gap-3">
        <FieldLabel htmlFor={id} className="text-[13px] font-medium text-ink">
          {label}
        </FieldLabel>
        <Switch
          id={id}
          checked={checked}
          onCheckedChange={onCheckedChange}
          disabled={disabled}
        />
      </div>
      {description ? (
        <FieldDescription className="text-[12px] leading-4">
          {description}
        </FieldDescription>
      ) : null}
    </Field>
  );
}
