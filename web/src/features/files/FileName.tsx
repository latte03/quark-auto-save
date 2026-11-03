import { useCallback, useLayoutEffect, useRef, useState } from "react";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "../../components/ui/tooltip";

function useTruncation(name: string) {
  const ref = useRef<HTMLElement | null>(null);
  const [truncated, setTruncated] = useState(false);

  const setRef = useCallback((element: HTMLElement | null) => {
    ref.current = element;
  }, []);

  useLayoutEffect(() => {
    const element = ref.current;
    if (!element) return;
    const update = () => {
      setTruncated(element.scrollWidth > element.clientWidth + 1);
    };
    update();
    const observer = new ResizeObserver(update);
    observer.observe(element);
    return () => observer.disconnect();
  }, [name]);

  return { setRef, truncated };
}

export function FileName({
  name,
  isDir,
  onOpen,
}: {
  name: string;
  isDir: boolean;
  onOpen: () => void;
}) {
  const { setRef, truncated } = useTruncation(name);
  const className =
    "min-w-0 flex-1 truncate text-left text-[13px] " +
    (isDir
      ? "font-medium text-ink hover:text-brand-ink hover:underline"
      : "text-ink");

  return (
    <Tooltip disabled={!truncated}>
      <TooltipTrigger
        render={
          isDir ? (
            <button ref={setRef} type="button" onClick={onOpen} className={className}>
              {name}
            </button>
          ) : (
            <span ref={setRef} className={className}>
              {name}
            </span>
          )
        }
      />
      <TooltipContent className="max-w-md break-all">{name}</TooltipContent>
    </Tooltip>
  );
}

export function TruncatedText({
  name,
  className,
}: {
  name: string;
  className?: string;
}) {
  const { setRef, truncated } = useTruncation(name);

  return (
    <Tooltip disabled={!truncated}>
      <TooltipTrigger
        render={
          <span ref={setRef} className={className}>
            {name}
          </span>
        }
      />
      <TooltipContent className="max-w-md break-all">{name}</TooltipContent>
    </Tooltip>
  );
}
