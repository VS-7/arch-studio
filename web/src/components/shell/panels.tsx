// Peças do layout de IDE: divisores arrastáveis e cabeçalhos de painel.

import { ChevronRight, Maximize2, Minimize2, Minus } from 'lucide-react'
import { useRef, useState, type ReactNode } from 'react'
import { cn } from '@/lib/utils'
import { IconAction } from '../ui'

/**
 * Divisor arrastável entre painéis.
 *
 * O novo tamanho é sempre `valor no início do arraste + deslocamento total do
 * ponteiro`, capturado em refs — nunca o valor de um render anterior. Com
 * pointer capture, o arraste continua mesmo quando o ponteiro passa por cima do
 * canvas ou sai da janela. Duplo clique restaura o tamanho padrão e as setas do
 * teclado ajustam em passos de 10px (Shift: 40px).
 */
export function Splitter({ axis, value, min, max, onChange, onReset, scale, invert, label }: {
  /** 'x' redimensiona larguras (linha vertical); 'y', alturas (linha horizontal). */
  axis: 'x' | 'y'
  value: number
  min: number
  max: number
  onChange: (value: number) => void
  onReset?: () => void
  /** Unidades de `value` por pixel — ex.: 1/altura quando `value` é uma proporção. */
  scale?: () => number
  /** O painel redimensionado fica depois do divisor (arrastar para trás aumenta). */
  invert?: boolean
  label: string
}) {
  const drag = useRef<{ start: number; value: number; scale: number } | null>(null)
  const [dragging, setDragging] = useState(false)
  const clamp = (v: number) => Math.min(max, Math.max(min, v))
  const pos = (e: React.PointerEvent) => (axis === 'x' ? e.clientX : e.clientY)
  const bodyClass = axis === 'x' ? 'resizing-x' : 'resizing-y'

  const start = (e: React.PointerEvent<HTMLDivElement>) => {
    if (e.button !== 0) return
    e.preventDefault()
    e.currentTarget.setPointerCapture(e.pointerId)
    drag.current = { start: pos(e), value, scale: scale?.() ?? 1 }
    setDragging(true)
    document.body.classList.add(bodyClass)
  }
  const move = (e: React.PointerEvent<HTMLDivElement>) => {
    const d = drag.current
    if (!d) return
    const delta = (pos(e) - d.start) * (invert ? -1 : 1)
    onChange(clamp(d.value + delta * d.scale))
  }
  const end = () => {
    if (!drag.current) return
    drag.current = null
    setDragging(false)
    document.body.classList.remove(bodyClass)
  }
  const key = (e: React.KeyboardEvent) => {
    const back = axis === 'x' ? 'ArrowLeft' : 'ArrowUp'
    const fwd = axis === 'x' ? 'ArrowRight' : 'ArrowDown'
    if (e.key !== back && e.key !== fwd) return
    e.preventDefault()
    const sign = (e.key === fwd ? 1 : -1) * (invert ? -1 : 1)
    onChange(clamp(value + sign * (e.shiftKey ? 40 : 10) * (scale?.() ?? 1)))
  }

  return (
    <div role="separator" tabIndex={0} aria-label={label}
      aria-orientation={axis === 'x' ? 'vertical' : 'horizontal'}
      aria-valuenow={Math.round(value * 100) / 100} aria-valuemin={min} aria-valuemax={max}
      onPointerDown={start} onPointerMove={move} onPointerUp={end} onPointerCancel={end} onLostPointerCapture={end}
      onDoubleClick={onReset} onKeyDown={key}
      className={cn('group/split relative z-20 shrink-0 touch-none bg-border outline-none',
        axis === 'x' ? 'w-px cursor-col-resize' : 'h-px cursor-row-resize')}>
      {/* Área de captura bem mais larga que a linha de 1px. */}
      <span className={cn('absolute', axis === 'x' ? 'inset-y-0 -inset-x-1' : 'inset-x-0 -inset-y-1')} />
      {/* Realce ao passar o mouse (com pequeno atraso), no foco e durante o arraste. */}
      <span className={cn(
        'pointer-events-none absolute bg-ring opacity-0 transition-opacity',
        'group-hover/split:opacity-100 group-hover/split:delay-150 group-focus-visible/split:opacity-100',
        axis === 'x' ? 'inset-y-0 -inset-x-px' : 'inset-x-0 -inset-y-px',
        dragging && 'opacity-100',
      )} />
    </div>
  )
}

/** Cabeçalho simples de painel lateral (Toolbox). */
export function PanelHeader({ title, children }: { title: string; children?: ReactNode }) {
  return (
    <div className="flex h-7 shrink-0 items-center justify-between border-b bg-panel-header px-2.5">
      <span className="text-[11px] font-semibold uppercase tracking-wide text-muted-foreground">{title}</span>
      <div className="flex items-center gap-0.5">{children}</div>
    </div>
  )
}

/**
 * Cabeçalho de seção empilhada (Model Explorer, Editor): clicar no título
 * recolhe/expande, como as views laterais do VS Code; o botão à direita maximiza
 * a seção dentro da barra lateral.
 */
export function SectionHeader({ title, open, maximized, onToggle, onMaximize, children }: {
  title: string; open: boolean; maximized: boolean
  onToggle: () => void; onMaximize: () => void; children?: ReactNode
}) {
  return (
    <div className="flex h-7 shrink-0 select-none items-center border-b bg-panel-header pl-1 pr-1.5">
      <button type="button" onClick={onToggle} aria-expanded={open}
        className="flex h-full min-w-0 flex-1 items-center gap-1 text-left outline-none focus-visible:underline">
        <ChevronRight size={13} className={cn('shrink-0 text-muted-foreground transition-transform', open && 'rotate-90')} />
        <span className="truncate text-[11px] font-semibold uppercase tracking-wide text-muted-foreground">{title}</span>
      </button>
      <div className="flex items-center gap-0.5">
        {children}
        {open && (
          <IconAction label="Recolher" onClick={onToggle}><Minus size={13} /></IconAction>
        )}
        <IconAction label={maximized ? 'Restaurar tamanho' : 'Maximizar'} onClick={onMaximize}>
          {maximized ? <Minimize2 size={12} /> : <Maximize2 size={12} />}
        </IconAction>
      </div>
    </div>
  )
}
