import { cn } from '@/lib/utils'

interface TerminalBlockProps {
  title?: string
  children: React.ReactNode
  className?: string
}

export function TerminalBlock({ title = 'Terminal', children, className }: TerminalBlockProps) {
  return (
    <div className={cn('overflow-hidden rounded-lg border border-border', className)}>
      <div className="flex items-center gap-2 border-b border-border bg-surface px-4 py-2.5">
        <div className="flex gap-1.5">
          <span className="h-3 w-3 rounded-full bg-terminal-red" />
          <span className="h-3 w-3 rounded-full bg-terminal-yellow" />
          <span className="h-3 w-3 rounded-full bg-terminal-green" />
        </div>
        <span className="ml-2 text-xs text-muted-foreground font-mono">{title}</span>
      </div>
      <div className="bg-code-bg p-4 font-mono text-sm leading-relaxed text-code-text overflow-x-auto">
        {children}
      </div>
    </div>
  )
}
