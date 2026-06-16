import { cn } from '@/lib/utils'

interface CodeBlockProps {
  code: string
  language?: string
  showLineNumbers?: boolean
  className?: string
}

export function CodeBlock({ code, showLineNumbers = true, className }: CodeBlockProps) {
  const lines = code.split('\n')

  return (
    <div className={cn('overflow-hidden rounded-lg border border-border', className)}>
      <div className="overflow-x-auto bg-code-bg p-4">
        <pre className="font-mono text-sm leading-relaxed">
          {lines.map((line, i) => (
            <div key={i} className="flex">
              {showLineNumbers && (
                <span className="mr-4 inline-block w-8 select-none text-right text-muted-foreground/50">
                  {i + 1}
                </span>
              )}
              <span className="text-code-text">{line}</span>
            </div>
          ))}
        </pre>
      </div>
    </div>
  )
}
