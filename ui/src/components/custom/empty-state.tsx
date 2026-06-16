import type { LucideIcon } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

interface EmptyStateProps {
  icon: LucideIcon
  title: string
  description: string
  action?: {
    label: string
    onClick: () => void
  }
  className?: string
}

export function EmptyState({ icon: Icon, title, description, action, className }: EmptyStateProps) {
  return (
    <div className={cn('flex flex-col items-center justify-center py-16 text-center', className)}>
      <div className="relative mb-5">
        {/* Ambient glow behind icon */}
        <div className="absolute inset-0 rounded-full bg-brand/10 blur-xl scale-150" />
        <div className="relative rounded-2xl bg-gradient-to-br from-brand/10 to-brand/5 border border-brand/15 p-5">
          <Icon className="h-8 w-8 text-brand" />
        </div>
      </div>
      <h3 className="mb-1.5 text-lg font-semibold text-foreground">{title}</h3>
      <p className="mb-6 max-w-sm text-sm text-muted-foreground leading-relaxed">{description}</p>
      {action && (
        <Button onClick={action.onClick} className="shadow-[0_0_20px_rgba(255,107,53,0.2)]">
          {action.label}
        </Button>
      )}
    </div>
  )
}
