import { useState } from "react";
import { glossary } from "@/lib/glossary";
import {
  Popover,
  PopoverTrigger,
  PopoverContent,
} from "@/components/ui/popover";
import { cn } from "@/lib/utils";

interface HelpTooltipProps {
  /** Key from glossary.ts */
  term: string;
  /** Override the glossary text */
  text?: string;
  /** Visual size of the ? icon */
  size?: "sm" | "md";
  className?: string;
}

export function HelpTooltip({
  term,
  text,
  size = "sm",
  className,
}: HelpTooltipProps) {
  const [open, setOpen] = useState(false);
  const entry = glossary[term];
  const plain = text ?? entry?.plain;

  if (!plain) return null;

  const iconSize = size === "sm" ? "h-3.5 w-3.5 text-[9px]" : "h-4 w-4 text-[10px]";

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <button
          type="button"
          aria-label={`Learn more about ${entry?.term ?? term}`}
          onClick={() => setOpen(!open)}
          onMouseEnter={() => setOpen(true)}
          onMouseLeave={() => setOpen(false)}
          onFocus={() => setOpen(true)}
          onBlur={() => setOpen(false)}
          className={cn(
            "inline-flex items-center justify-center rounded-full",
            "border border-muted-foreground/25 text-muted-foreground/50",
            "hover:border-brand/40 hover:text-brand/70",
            "focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-brand/50",
            "transition-colors cursor-help shrink-0",
            iconSize,
            className,
          )}
        >
          ?
        </button>
      </PopoverTrigger>
      <PopoverContent
        side="top"
        align="center"
        sideOffset={6}
        className={cn(
          "max-w-[280px] rounded-lg border border-border/50",
          "bg-popover p-3 shadow-lg",
          "text-xs leading-relaxed",
        )}
        onMouseEnter={() => setOpen(true)}
        onMouseLeave={() => setOpen(false)}
      >
        {entry?.term && (
          <p className="font-semibold text-foreground mb-1">{entry.term}</p>
        )}
        <p className="text-muted-foreground">{plain}</p>
        {entry?.example && (
          <p className="mt-1.5 text-[10px] text-muted-foreground/60 font-mono">
            {entry.example}
          </p>
        )}
      </PopoverContent>
    </Popover>
  );
}
