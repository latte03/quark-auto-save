import { useEffect, useId, useState } from "react";
import { Field, FieldLabel } from "./ui/field";
import { Input } from "./ui/input";

export function PluginTextField({
  label,
  value,
  onChange,
  placeholder,
  secret = false,
}: {
  label: string;
  value: string;
  onChange: (next: string) => void;
  placeholder?: string;
  secret?: boolean;
}) {
  const id = useId();
  return (
    <Field className="py-1.5">
      <FieldLabel htmlFor={id}>{label}</FieldLabel>
      <Input
        id={id}
        type={secret ? "password" : "text"}
        value={value}
        placeholder={placeholder}
        onChange={(event) => onChange(event.target.value)}
      />
    </Field>
  );
}

export function PluginNumberField({
  label,
  value,
  onChange,
}: {
  label: string;
  value: number;
  onChange: (next: number) => void;
}) {
  const id = useId();
  // 数字输入保留本地草稿：直接 Number("") 会立刻写成 0。合法值即时写回，
  // 这样脏标记和关面板丢弃都不会漏；非法内容在失焦时回退到上一个值。
  const [text, setText] = useState(String(value));

  useEffect(() => {
    if (Number(text) !== value) setText(String(value));
    // 只跟随外部值变化，避免输入过程中的 "1." 被立刻改写。
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [value]);

  const commit = (raw: string) => {
    const parsed = Number(raw.trim());
    // 插件里的数字都是计数；小数或 0 会让后端按整型解码时整段配置退回默认。
    if (raw.trim() === "" || !Number.isInteger(parsed) || parsed < 1) return false;
    onChange(parsed);
    return true;
  };

  return (
    <Field className="py-1.5">
      <FieldLabel htmlFor={id}>{label}</FieldLabel>
      <Input
        id={id}
        inputMode="numeric"
        value={text}
        onChange={(event) => {
          setText(event.target.value);
          commit(event.target.value);
        }}
        onBlur={() => {
          if (!commit(text)) setText(String(value));
        }}
        onKeyDown={(event) => {
          if (event.key === "Enter") event.currentTarget.blur();
        }}
      />
    </Field>
  );
}
