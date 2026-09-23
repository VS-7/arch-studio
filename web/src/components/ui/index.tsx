// Primitivas de interface compartilhadas. Mantidas propositalmente pequenas e
// sem dependências de design system externo, para não inflar o bundle embutido
// no binário Go.

import { X, Loader2, type LucideIcon } from 'lucide-react'
import {
  createContext, useCallback, useContext, useEffect, useMemo, useRef, useState,
  type ButtonHTMLAttributes, type InputHTMLAttributes, type ReactNode,
  type SelectHTMLAttributes, type TextareaHTMLAttributes,
} from 'react'

export function cx(...parts: (string | false | null | undefined)[]): string {
  return parts.filter(Boolean).join(' ')
}

/* -------------------------------------------------------------------------- */
/* Button                                                                      */
/* -------------------------------------------------------------------------- */

type ButtonVariant = 'primary' | 'secondary' | 'ghost' | 'danger' | 'success'
type ButtonSize = 'sm' | 'md' | 'lg' | 'icon'

const BUTTON_VARIANTS: Record<ButtonVariant, string> = {
  primary: 'bg-sky-500 text-white hover:bg-sky-400 active:bg-sky-600 shadow-sm shadow-sky-500/25',
  secondary: 'surface-3 text-app border border-app hover:brightness-110',
  ghost: 'text-muted-app hover:surface-3 hover:text-app',
  danger: 'bg-rose-500/90 text-white hover:bg-rose-500',
  success: 'bg-emerald-500 text-white hover:bg-emerald-400',
}

const BUTTON_SIZES: Record<ButtonSize, string> = {
  sm: 'h-7 px-2.5 text-xs gap-1.5',
  md: 'h-9 px-3.5 text-sm gap-2',
  lg: 'h-11 px-5 text-base gap-2',
  icon: 'h-8 w-8 justify-center',
}

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant
  size?: ButtonSize
  icon?: LucideIcon
  loading?: boolean
}

export function Button({
  variant = 'secondary', size = 'md', icon: Icon, loading, className, children, disabled, ...rest
}: ButtonProps) {
  return (
    <button
      {...rest}
      disabled={disabled || loading}
      className={cx(
        'inline-flex items-center rounded-lg font-medium transition-all select-none',
        'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sky-500/60',
        'disabled:opacity-45 disabled:pointer-events-none',
        BUTTON_VARIANTS[variant], BUTTON_SIZES[size], className,
      )}
    >
      {loading ? <Loader2 size={size === 'sm' ? 13 : 15} className="animate-spin" /> : Icon ? <Icon size={size === 'sm' ? 13 : 15} /> : null}
      {children}
    </button>
  )
}

/* -------------------------------------------------------------------------- */
/* Campos de formulário                                                        */
/* -------------------------------------------------------------------------- */

const FIELD_BASE =
  'w-full rounded-lg border border-app surface px-3 text-sm text-app placeholder:text-muted-app/60 ' +
  'transition-colors focus:outline-none focus:border-sky-500 focus:ring-2 focus:ring-sky-500/20'

export function Input({ className, ...rest }: InputHTMLAttributes<HTMLInputElement>) {
  return <input {...rest} className={cx(FIELD_BASE, 'h-9', className)} />
}

export function Textarea({ className, ...rest }: TextareaHTMLAttributes<HTMLTextAreaElement>) {
  return <textarea {...rest} className={cx(FIELD_BASE, 'py-2 leading-relaxed resize-y', className)} />
}

export function Select({ className, children, ...rest }: SelectHTMLAttributes<HTMLSelectElement>) {
  return (
    <select {...rest} className={cx(FIELD_BASE, 'h-9 cursor-pointer appearance-none pr-8', className)}
      style={{
        backgroundImage:
          "url(\"data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='12' height='12' viewBox='0 0 24 24' fill='none' stroke='%2393a1bd' stroke-width='3'%3E%3Cpath d='M6 9l6 6 6-6'/%3E%3C/svg%3E\")",
        backgroundRepeat: 'no-repeat',
        backgroundPosition: 'right 10px center',
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
    <label className={cx('block', className)}>
      <span className="mb-1.5 block text-[11px] font-semibold uppercase tracking-wider text-muted-app">{label}</span>
      {children}
      {hint && <span className="mt-1 block text-[11px] text-muted-app">{hint}</span>}
    </label>
  )
}

/* -------------------------------------------------------------------------- */
/* Apresentação                                                                */
/* -------------------------------------------------------------------------- */

export function Badge({ children, color, className }: { children: ReactNode; color?: string; className?: string }) {
  return (
    <span
      className={cx('inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[10.5px] font-semibold', className)}
      style={color ? { backgroundColor: `${color}1f`, color, border: `1px solid ${color}40` } : undefined}
    >
      {children}
    </span>
  )
}

export function Card({ title, actions, children, className, dense }: {
  title?: ReactNode; actions?: ReactNode; children: ReactNode; className?: string; dense?: boolean
}) {
  return (
    <section className={cx('surface rounded-xl border border-app', className)}>
      {(title || actions) && (
        <header className="flex items-center justify-between gap-3 border-b border-app px-4 py-2.5">
          <h3 className="text-sm font-semibold text-app">{title}</h3>
          {actions && <div className="flex items-center gap-1.5">{actions}</div>}
        </header>
      )}
      <div className={dense ? '' : 'p-4'}>{children}</div>
    </section>
  )
}

export function Stat({ label, value, sub, accent }: {
  label: string; value: ReactNode; sub?: ReactNode; accent?: string
}) {
  return (
    <div className="surface rounded-xl border border-app px-4 py-3">
      <div className="text-[11px] font-semibold uppercase tracking-wider text-muted-app">{label}</div>
      <div className="mt-1 text-2xl font-bold tabular-nums" style={accent ? { color: accent } : undefined}>{value}</div>
      {sub && <div className="mt-0.5 text-xs text-muted-app">{sub}</div>}
    </div>
  )
}

export function EmptyState({ icon: Icon, title, description, action }: {
  icon: LucideIcon; title: string; description?: string; action?: ReactNode
}) {
  return (
    <div className="flex flex-col items-center justify-center gap-3 px-6 py-14 text-center">
      <div className="surface-3 rounded-2xl p-3.5 text-muted-app"><Icon size={26} /></div>
      <div>
        <p className="text-sm font-semibold text-app">{title}</p>
        {description && <p className="mt-1 max-w-md text-xs leading-relaxed text-muted-app">{description}</p>}
      </div>
      {action}
    </div>
  )
}

export function Spinner({ label }: { label?: string }) {
  return (
    <div className="flex items-center justify-center gap-2 py-10 text-sm text-muted-app">
      <Loader2 size={16} className="animate-spin" />
      {label}
    </div>
  )
}

/* -------------------------------------------------------------------------- */
/* Modal                                                                       */
/* -------------------------------------------------------------------------- */

export function Modal({ open, onClose, title, description, children, footer, wide }: {
  open: boolean; onClose: () => void; title: string; description?: string
  children: ReactNode; footer?: ReactNode; wide?: boolean
}) {
  useEffect(() => {
    if (!open) return
    const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose() }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [open, onClose])

  if (!open) return null
  return (
    <div className="fixed inset-0 z-50 flex items-start justify-center overflow-y-auto bg-black/55 p-4 backdrop-blur-sm"
      onMouseDown={(e) => { if (e.target === e.currentTarget) onClose() }}>
      <div className={cx('surface my-8 w-full rounded-2xl border border-app shadow-2xl', wide ? 'max-w-4xl' : 'max-w-xl')}>
        <header className="flex items-start justify-between gap-4 border-b border-app px-5 py-3.5">
          <div>
            <h2 className="text-base font-semibold text-app">{title}</h2>
            {description && <p className="mt-0.5 text-xs text-muted-app">{description}</p>}
          </div>
          <Button variant="ghost" size="icon" onClick={onClose} aria-label="Fechar"><X size={16} /></Button>
        </header>
        <div className="px-5 py-4">{children}</div>
        {footer && <footer className="flex justify-end gap-2 border-t border-app px-5 py-3">{footer}</footer>}
      </div>
    </div>
  )
}

/* -------------------------------------------------------------------------- */
/* Toasts                                                                      */
/* -------------------------------------------------------------------------- */

export type ToastKind = 'info' | 'success' | 'error' | 'ai'

interface Toast { id: number; kind: ToastKind; message: string }

const ToastContext = createContext<(kind: ToastKind, message: string) => void>(() => {})

export function useToast() { return useContext(ToastContext) }

const TOAST_STYLES: Record<ToastKind, string> = {
  info: 'border-sky-500/40 bg-sky-500/10 text-sky-200',
  success: 'border-emerald-500/40 bg-emerald-500/10 text-emerald-200',
  error: 'border-rose-500/40 bg-rose-500/10 text-rose-200',
  ai: 'border-violet-500/40 bg-violet-500/10 text-violet-200',
}

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([])
  const seq = useRef(0)

  const push = useCallback((kind: ToastKind, message: string) => {
    const id = ++seq.current
    setToasts((prev) => [...prev.slice(-4), { id, kind, message }])
    setTimeout(() => setToasts((prev) => prev.filter((t) => t.id !== id)), kind === 'error' ? 7000 : 3800)
  }, [])

  const value = useMemo(() => push, [push])

  return (
    <ToastContext.Provider value={value}>
      {children}
      <div className="pointer-events-none fixed bottom-5 right-5 z-[60] flex w-80 flex-col gap-2">
        {toasts.map((t) => (
          <div key={t.id}
            className={cx('pointer-events-auto rounded-xl border px-3.5 py-2.5 text-xs font-medium shadow-lg backdrop-blur-md', TOAST_STYLES[t.kind])}>
            {t.message}
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
          <span className="w-5 shrink-0 text-right text-[11px] tabular-nums text-muted-app">
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
