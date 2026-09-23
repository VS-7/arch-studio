// Moldura das telas abertas em abas (documentação, contratos, precificação…):
// uma barra de ferramentas colada à aba, com título, resumo e ações, e o
// conteúdo ocupando todo o resto da área central — sem cartões flutuando no
// meio de margens vazias.

import type { LucideIcon } from 'lucide-react'
import type { ReactNode } from 'react'
import { cn } from '@/lib/utils'

export function ViewFrame({ icon: Icon, title, meta, actions, children, bodyClassName }: {
  icon: LucideIcon
  title: ReactNode
  meta?: ReactNode
  actions?: ReactNode
  children: ReactNode
  /** Substitui a rolagem padrão do corpo (ex.: layouts com painel lateral próprio). */
  bodyClassName?: string
}) {
  return (
    <div className="flex h-full min-h-0 flex-col bg-background">
      <header className="flex min-h-11 shrink-0 flex-wrap items-center gap-x-3 gap-y-1.5 border-b px-3 py-1.5">
        <span className="flex size-7 shrink-0 items-center justify-center rounded-sm border bg-muted text-muted-foreground">
          <Icon size={15} />
        </span>
        <div className="min-w-[10rem] flex-1">
          <h2 className="truncate text-[13px] font-semibold leading-tight">{title}</h2>
          {meta && <p className="truncate text-[11.5px] leading-snug text-muted-foreground">{meta}</p>}
        </div>
        {actions && <div className="flex flex-wrap items-center gap-1.5">{actions}</div>}
      </header>
      {/* @container: colunas reagem à largura útil da área central, não da janela. */}
      <div className={cn('@container min-h-0 flex-1', bodyClassName ?? 'overflow-y-auto')}>
        {children}
      </div>
    </div>
  )
}
