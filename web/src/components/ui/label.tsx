import * as LabelPrimitive from '@radix-ui/react-label'
import * as React from 'react'
import { cn } from '@/lib/utils'

function Label({ className, ...props }: React.ComponentProps<typeof LabelPrimitive.Root>) {
  return (
    <LabelPrimitive.Root
      data-slot="label"
      className={cn('flex select-none items-center gap-2 text-xs font-medium leading-none text-muted-foreground peer-disabled:opacity-50', className)}
      {...props}
    />
  )
}

export { Label }
