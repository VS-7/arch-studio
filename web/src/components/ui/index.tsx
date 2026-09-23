// Primitivas de interface compartilhadas pelas views.
//
// Construídas sobre os componentes shadcn/ui deste diretório (button.tsx,
// dialog.tsx, …). Esta camada preserva uma API curta e estável para as telas —
// `<Button variant="primary" icon={Save}>` — enquanto o visual vem inteiro do
// design system.

import { Loader2, X, type LucideIcon } from 'lucide-react'
import {
  createContext, useCallback, useContext, useMemo, useRef, useState,
  type ButtonHTMLAttributes, type InputHTMLAttributes, type ReactNode,
  type SelectHTMLAttributes, type TextareaHTMLAttributes,
} from 'react'
import { cn } from '@/lib/utils'
import { Button as UIButton } from './button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from './dialog'
import { Input as UIInput } from './input'
import { Textarea as UITextarea } from './textarea'
import { Tooltip, TooltipContent, TooltipTrigger } from './tooltip'

/** Alias histórico de `cn`. */
export function cx(...parts: (string | false | null | undefined)[]): string {
  return cn(...parts)
}

/* -------------------------------------------------------------------------- */
/* Button                                                                      */
/* -------------------------------------------------------------------------- */

type ButtonVariant = 'primary' | 'secondary' | 'ghost' | 'danger' | 'success' | 'outline'
type ButtonSize = 'sm' | 'md' | 'lg' | 'icon'

const VARIANT_MAP = {
  primary: 'default', secondary: 'secondary', ghost: 'ghost', danger: 'destructive',
  success: 'success', outline: 'outline',
} as const

const SIZE_MAP = { sm: 'sm', md: 'default', lg: 'lg', icon: 'icon-sm' } as const

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant
  size?: ButtonSize
  icon?: LucideIcon
  loading?: boolean
}

export function Button({
  variant = 'secondary', size = 'md', icon: Icon, loading, children, disabled, title, ...rest
}: ButtonProps) {
  // Botões só com ícone ganham tooltip do design system em vez do `title` nativo.
  const tip = size === 'icon' ? title ?? rest['aria-label'] : undefined
  const button = (
    <UIButton {...rest} title={tip ? undefined : title} aria-label={rest['aria-label'] ?? tip}
      variant={VARIANT_MAP[variant]} size={SIZE_MAP[size]} disabled={disabled || loading}>
      {loading ? <Loader2 className="animate-spin" /> : Icon ? <Icon /> : null}
      {children}
    </UIButton>
  )
  return tip ? <Tip label={tip}>{button}</Tip> : button
}

/** Tooltip do shadcn/ui em volta de qualquer elemento interativo. */
export function Tip({ label, children, side }: {
  label: ReactNode; children: React.ReactElement; side?: 'top' | 'right' | 'bottom' | 'left'
}) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>{children}</TooltipTrigger>
      <TooltipContent side={side}>{label}</TooltipContent>
    </Tooltip>
  )
}

/** Botão compacto só com ícone (painéis, árvores, cabeçalhos), sempre com tooltip. */
export function IconAction({ label, onClick, children, className, side }: {
  label: string; onClick: (e: React.MouseEvent) => void; children: ReactNode; className?: string
  side?: 'top' | 'right' | 'bottom' | 'left'
}) {
  return (
    <Tip label={label} side={side}>
      <button type="button" aria-label={label} onClick={onClick}
        className={cn('flex items-center justify-center rounded-sm p-0.5 text-muted-foreground hover:bg-accent hover:text-foreground', className)}>
        {children}
      </button>
    </Tip>
  )
}

/* -------------------------------------------------------------------------- */
/* Campos de formulário                                                        */
/* -------------------------------------------------------------------------- */

export function Input(props: InputHTMLAttributes<HTMLInputElement>) {
  return <UIInput {...props} />
}

export function Textarea(props: TextareaHTMLAttributes<HTMLTextAreaElement>) {
  return <UITextarea {...props} />
}

/** Select nativo com o visual do Input (equivalente ao native-select do shadcn). */
export function Select({ className, children, ...rest }: SelectHTMLAttributes<HTMLSelectElement>) {
  return (
    <select
      {...rest}
      className={cn(
        'h-8 w-full cursor-pointer appearance-none rounded-md border border-input bg-background pl-2.5 pr-7 text-[13px] text-foreground shadow-xs outline-none',
        'focus-visible:border-ring focus-visible:ring-2 focus-visible:ring-ring/25 disabled:opacity-50',
        className,
      )}
      style={{
        backgroundImage:
          "url(\"data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='12' height='12' viewBox='0 0 24 24' fill='none' stroke='%238a9099' stroke-width='2.5'%3E%3Cpath d='M6 9l6 6 6-6'/%3E%3C/svg%3E\")",
        backgroundRepeat: 'no-repeat',
        backgroundPosition: 'right 8px center',
      }}
    >
      {children}
    </select>
  )
}

export function Field({ label, hint, children, className }: {
  label: string; hint?: string; children: ReactNode; className?: string
}) {
  return (
    <label className={cn('block', className)}>
      <span className="mb-1 block text-[11.5px] font-medium text-muted-foreground">{label}</span>
      {children}
      {hint && <span className="mt-1 block text-[11px] text-muted-foreground/80">{hint}</span>}
    </label>
  )
}

/* -------------------------------------------------------------------------- */
/* Apresentação                                                                */
/* -------------------------------------------------------------------------- */

export function Badge({ children, color, className }: { children: ReactNode; color?: string; className?: string }) {
  return (
    <span
      className={cn(
        'inline-flex items-center gap-1 rounded-md border px-1.5 py-0.5 text-[10.5px] font-semibold',
        !color && 'bg-secondary text-secondary-foreground',
        className,
      )}
      style={color ? { backgroundColor: `color-mix(in oklab, ${color} 12%, transparent)`, color, borderColor: `color-mix(in oklab, ${color} 30%, transparent)` } : undefined}
    >
      {children}
    </span>
  )
}

export function Card({ title, actions, children, className, dense }: {
  title?: ReactNode; actions?: ReactNode; children: ReactNode; className?: string; dense?: boolean
}) {
  return (
    <section className={cn('rounded-lg border bg-card text-card-foreground shadow-xs', className)}>
      {(title || actions) && (
        <header className="flex items-center justify-between gap-3 border-b bg-panel-header/60 px-3.5 py-2">
          <h3 className="text-[12.5px] font-semibold">{title}</h3>
          {actions && <div className="flex items-center gap-1.5">{actions}</div>}
        </header>
      )}
      <div className={dense ? '' : 'p-3.5'}>{children}</div>
    </section>
  )
}

export function Stat({ label, value, sub, accent }: {
  label: string; value: ReactNode; sub?: ReactNode; accent?: string
}) {
  return (
    // Container query: o valor diminui em cards estreitos em vez de quebrar linha.
    <div className="@container min-w-0 rounded-lg border bg-card px-3.5 py-3 shadow-xs">
      <div className="truncate text-[11px] font-medium text-muted-foreground">{label}</div>
      <div className="mt-1 truncate text-lg font-semibold tabular-nums @[13rem]:text-2xl" style={accent ? { color: accent } : undefined}
        title={typeof value === 'string' ? value : undefined}>{value}</div>
      {sub && <div className="mt-0.5 truncate text-xs text-muted-foreground" title={typeof sub === 'string' ? sub : undefined}>{sub}</div>}
    </div>
  )
}

export function EmptyState({ icon: Icon, title, description, action }: {
  icon: LucideIcon; title: string; description?: string; action?: ReactNode
}) {
  return (
    <div className="flex flex-col items-center justify-center gap-3 px-6 py-14 text-center">
      <div className="rounded-lg border bg-muted p-3 text-muted-foreground"><Icon size={24} /></div>
      <div>
        <p className="text-sm font-semibold">{title}</p>
        {description && <p className="mt-1 max-w-md text-xs leading-relaxed text-muted-foreground">{description}</p>}
      </div>
      {action}
    </div>
  )
}

export function Spinner({ label }: { label?: string }) {
  return (
    <div className="flex items-center justify-center gap-2 py-10 text-sm text-muted-foreground">
      <Loader2 size={16} className="animate-spin" />
      {label}
    </div>
  )
}

/* -------------------------------------------------------------------------- */
/* Modal (Dialog do shadcn)                                                    */
/* -------------------------------------------------------------------------- */

export function Modal({ open, onClose, title, description, children, footer, wide }: {
  open: boolean; onClose: () => void; title: string; description?: string
  children: ReactNode; footer?: ReactNode; wide?: boolean
}) {
  return (
    <Dialog open={open} onOpenChange={(next) => { if (!next) onClose() }}>
      <DialogContent className={wide ? 'sm:max-w-4xl' : 'sm:max-w-xl'}>
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          {description ? <DialogDescription>{description}</DialogDescription> : <DialogDescription className="sr-only">{title}</DialogDescription>}
        </DialogHeader>
        <div className="min-w-0">{children}</div>
        {footer && <DialogFooter className="border-t pt-3">{footer}</DialogFooter>}
      </DialogContent>
    </Dialog>
  )
}

/* -------------------------------------------------------------------------- */
/* Toasts (no estilo do Sonner usado pelo shadcn)                              */
/* -------------------------------------------------------------------------- */

export type ToastKind = 'info' | 'success' | 'error' | 'ai'

/** Ação opcional exibida no toast (ex.: "Desfazer"). */
export interface ToastAction { label: string; onClick: () => void }

interface Toast { id: number; kind: ToastKind; message: string; action?: ToastAction }

type ToastFn = (kind: ToastKind, message: string, action?: ToastAction) => void

const ToastContext = createContext<ToastFn>(() => {})

export function useToast() { return useContext(ToastContext) }

const TOAST_ACCENT: Record<ToastKind, string> = {
  info: 'var(--info)', success: 'var(--success)', error: 'var(--destructive)', ai: 'var(--ai)',
}

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([])
  const seq = useRef(0)

  const push = useCallback<ToastFn>((kind, message, action) => {
    const id = ++seq.current
    setToasts((prev) => [...prev.slice(-4), { id, kind, message, action }])
    // Toasts com ação ficam mais tempo na tela, para dar tempo de desfazer.
    setTimeout(() => setToasts((prev) => prev.filter((t) => t.id !== id)), kind === 'error' ? 7000 : action ? 6000 : 3800)
  }, [])

  const value = useMemo(() => push, [push])

  return (
    <ToastContext.Provider value={value}>
      {children}
      {/* Topo central, abaixo da barra de menus e das abas: não cobre Toolbox, Editor,
          controles do canvas nem a barra de status. */}
      <div className="pointer-events-none fixed left-1/2 top-[4.75rem] z-[60] flex w-[min(26rem,calc(100vw-2rem))] -translate-x-1/2 flex-col items-center gap-2">
        {toasts.map((t) => (
          <div key={t.id} role="status"
            className="pointer-events-auto flex w-full items-start gap-2.5 rounded-md border bg-popover px-3 py-2.5 text-[12.5px] text-popover-foreground shadow-lg animate-in fade-in-0 slide-in-from-top-2">
            <span className="mt-1 size-2 shrink-0 rounded-full" style={{ backgroundColor: TOAST_ACCENT[t.kind] }} />
            <span className="min-w-0 flex-1 break-words">{t.message}</span>
            {t.action && (
              <button className="shrink-0 rounded-sm px-1.5 text-[12px] font-semibold underline-offset-2 hover:underline"
                onClick={() => { t.action!.onClick(); setToasts((prev) => prev.filter((x) => x.id !== t.id)) }}>
                {t.action.label}
              </button>
            )}
            <button className="text-muted-foreground hover:text-foreground" aria-label="Fechar"
              onClick={() => setToasts((prev) => prev.filter((x) => x.id !== t.id))}>
              <X size={13} />
            </button>
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  )
}

/* -------------------------------------------------------------------------- */
/* Lista editável de strings (fluxos, pré-condições, critérios…)               */
/* -------------------------------------------------------------------------- */

export function StringList({ value, onChange, placeholder, ordered }: {
  value: string[]; onChange: (next: string[]) => void; placeholder?: string; ordered?: boolean
}) {
  const items = value.length ? value : ['']
  const update = (index: number, text: string) => {
    const next = [...items]
    next[index] = text
    onChange(next.filter((v, i) => v.trim() !== '' || i < next.length - 1))
  }
  return (
    <div className="space-y-1.5">
      {items.map((item, index) => (
        <div key={index} className="flex items-center gap-2">
          <span className="w-5 shrink-0 text-right text-[11px] tabular-nums text-muted-foreground">
            {ordered ? `${index + 1}.` : '•'}
          </span>
          <Input
            value={item}
            placeholder={placeholder}
            onChange={(e) => update(index, e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') {
                e.preventDefault()
                const next = [...items]
                next.splice(index + 1, 0, '')
                onChange(next)
              }
            }}
          />
          <Button variant="ghost" size="icon" aria-label="Remover item"
            onClick={() => onChange(items.filter((_, i) => i !== index))}>
            <X size={13} />
          </Button>
        </div>
      ))}
      <Button variant="ghost" size="sm" onClick={() => onChange([...items, ''])}>+ adicionar</Button>
    </div>
  )
}
