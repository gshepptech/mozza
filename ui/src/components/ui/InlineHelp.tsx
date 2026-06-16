import { useState, type ReactNode } from "react";
import { cn } from "@/lib/utils";

interface InlineHelpProps {
  children: ReactNode;
  /** Override the toggle label */
  label?: string;
  /** Start expanded */
  defaultOpen?: boolean;
  className?: string;
}

export function InlineHelp({
  children,
  label = "Learn more",
  defaultOpen = false,
  className,
}: InlineHelpProps) {
  const [open, setOpen] = useState(defaultOpen);

  return (
    <div className={cn("mt-1", className)}>
      <button
        type="button"
        onClick={() => setOpen(!open)}
        className={cn(
          "text-[11px] text-muted-foreground/60 hover:text-muted-foreground",
          "transition-colors focus-visible:outline-none focus-visible:underline",
        )}
      >
        {open ? "Hide" : label}
      </button>
      <div
        className={cn(
          "grid transition-[grid-template-rows] duration-200 ease-out",
          open ? "grid-rows-[1fr]" : "grid-rows-[0fr]",
        )}
      >
        <div className="overflow-hidden">
          <p className="pt-1.5 text-xs text-muted-foreground/70 leading-relaxed">
            {children}
          </p>
        </div>
      </div>
    </div>
  );
}
