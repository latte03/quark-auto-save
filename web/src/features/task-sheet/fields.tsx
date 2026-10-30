import { Plus, X } from "lucide-react";
import { Button } from "../../components/ui/button";
import { Field, FieldLabel } from "../../components/ui/field";
import { Input } from "../../components/ui/input";
import { SwitchField } from "../../components/SwitchField";
import { pluginFieldLabels } from "./constants";

export function RunTimesField({
  value,
  onChange,
}: {
  value: string[];
  onChange: (next: string[]) => void;
}) {
  return (
    <div className="space-y-2">
      {value.map((clock, index) => (
        <div key={index} className="flex items-center gap-1.5">
          <Input
            type="time"
            value={clock}
            aria-label={`运行时间 ${index + 1}`}
            className="w-32 font-mono tabular-nums"
            onChange={(event) => {
              const next = [...value];
              next[index] = event.target.value;
              onChange(next);
            }}
            onBlur={() => {
              const current = value[index] ?? "";
              if (!current || value.indexOf(current) !== index) {
                onChange(value.filter((_, cursor) => cursor !== index));
              }
            }}
          />
          <Button
            type="button"
            variant="ghost"
            size="icon-sm"
            aria-label={`删除 ${clock || "该时刻"}`}
            onClick={() => onChange(value.filter((_, cursor) => cursor !== index))}
          >
            <X className="h-3.5 w-3.5" />
          </Button>
        </div>
      ))}
      <Button
        type="button"
        variant="secondary"
        size="sm"
        onClick={() => onChange([...value, "08:00"])}
      >
        <Plus className="h-3.5 w-3.5" />
        添加时刻
      </Button>
      {value.length === 0 ? (
        <p className="text-[12px] text-warn">没有运行时刻时，这个任务不会自动运行。</p>
      ) : null}
    </div>
  );
}

export function PluginFields({
  values,
  onChange,
}: {
  values: Record<string, string | number | boolean>;
  onChange: (key: string, value: string | number | boolean) => void;
}) {
  return (
    <>
      {Object.entries(values).map(([key, value]) => {
        if (typeof value === "boolean") {
          return (
            <SwitchField
              key={key}
              label={pluginFieldLabels[key] ?? key}
              checked={value}
              onCheckedChange={(checked) => onChange(key, checked)}
            />
          );
        }
        return (
          <Field key={key} className="py-1.5">
            <FieldLabel>{pluginFieldLabels[key] ?? key}</FieldLabel>
            <Input
              value={String(value)}
              inputMode={typeof value === "number" ? "numeric" : undefined}
              onChange={(event) =>
                onChange(
                  key,
                  typeof value === "number"
                    ? Number(event.target.value)
                    : event.target.value,
                )
              }
            />
          </Field>
        );
      })}
    </>
  );
}
