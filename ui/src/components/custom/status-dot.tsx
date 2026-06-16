import { cn } from '@/lib/utils'

type Status = 'success' | 'error' | 'warning' | 'info' | 'neutral'

const statusColors: Record<Status, string> = {
  success: 'bg-success',
  error: 'bg-error',
  warning: 'bg-warning',
  info: 'bg-info',
  neutral: 'bg-muted-foreground',
}

const pulseColors: Record<Status, string> = {
  success: 'bg-success',
  error: 'bg-error',
  warning: 'bg-warning',
  info: 'bg-info',
  neutral: 'bg-muted-foreground',
}

interface StatusDotProps {
  status: Status
  pulse?: boolean
  size?: 'sm' | 'md' | 'lg'
  className?: string
}

const sizes = {
  sm: 'h-1.5 w-1.5',
  md: 'h-2 w-2',
  lg: 'h-2.5 w-2.5',
}

export function StatusDot({ status, pulse, size = 'md', className }: StatusDotProps) {
  return (
    <span className={cn('relative inline-flex', className)}>
      {pulse && (
        <span
          className={cn(
            'absolute inline-flex h-full w-full animate-ping rounded-full opacity-75',
            pulseColors[status]
          )}
        />
      )}
      <span
        className={cn('relative inline-flex rounded-full', sizes[size], statusColors[status])}
      />
    </span>
  )
}
