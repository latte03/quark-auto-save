import { cn } from "../../lib/utils";
import { sections, type SettingsSection } from "./constants";

export function SettingsNav({
  section,
  onNavigate,
}: {
  section: SettingsSection;
  onNavigate: (id: SettingsSection) => void;
}) {
  return (
    <aside className="lg:sticky lg:top-5 lg:self-start">
      <div className="grid grid-cols-3 gap-1 lg:flex lg:flex-col lg:gap-0.5">
        {sections.map((item) => {
          const Icon = item.icon;
          const active = section === item.id;
          return (
            <button
              key={item.id}
              onClick={() => onNavigate(item.id)}
              aria-current={active ? "true" : undefined}
              className={cn(
                "relative flex items-center gap-2 rounded-md px-2 py-1.5 text-left transition-colors lg:px-2.5 lg:py-2",
                active
                  ? "bg-brand text-white"
                  : "text-muted-ink hover:bg-line-soft hover:text-ink",
              )}
            >
              <Icon className="h-3.5 w-3.5 shrink-0" strokeWidth={1.7} />
              <span className="min-w-0">
                <span
                  className={cn(
                    "block text-[12px]",
                    active ? "font-semibold" : "font-medium",
                  )}
                >
                  {item.label}
                </span>
                <span
                  className={cn(
                    "hidden text-[12px] lg:block",
                    active ? "text-white/70" : "text-subtle",
                  )}
                >
                  {item.description}
                </span>
              </span>
            </button>
          );
        })}
      </div>
    </aside>
  );
}
