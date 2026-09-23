// Menu de contexto do canvas (clique direito / duplo clique), posicionado no
// cursor. Usa o DropdownMenu do shadcn/ui com uma âncora invisível, o que dá
// submenus, navegação por teclado e o mesmo visual dos menus da aplicação.

import type { ReactNode } from 'react'
import {
  DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuLabel, DropdownMenuSeparator,
  DropdownMenuShortcut, DropdownMenuSub, DropdownMenuSubContent, DropdownMenuSubTrigger, DropdownMenuTrigger,
} from '../ui/dropdown-menu'

export type CanvasMenuItem =
  | { type: 'item'; label: string; icon?: ReactNode; shortcut?: string; onSelect: () => void; disabled?: boolean; destructive?: boolean }
  | { type: 'sub'; label: string; icon?: ReactNode; items: CanvasMenuItem[]; disabled?: boolean }
  | { type: 'label'; label: string }
  | { type: 'separator' }

export interface CanvasMenuState { x: number; y: number; items: CanvasMenuItem[] }

export function CanvasMenu({ menu, onClose }: { menu: CanvasMenuState | null; onClose: () => void }) {
  if (!menu) return null
  return (
    <DropdownMenu open onOpenChange={(open) => { if (!open) onClose() }} modal={false}>
      <DropdownMenuTrigger asChild>
        <span aria-hidden style={{ position: 'fixed', left: menu.x, top: menu.y, width: 0, height: 0 }} />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" side="bottom" sideOffset={2} className="min-w-[13rem]"
        onCloseAutoFocus={(e) => e.preventDefault()}>
        <Items items={menu.items} onClose={onClose} />
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

function Items({ items, onClose }: { items: CanvasMenuItem[]; onClose: () => void }) {
  return (
    <>
      {items.map((item, i) => {
        switch (item.type) {
          case 'separator':
            return <DropdownMenuSeparator key={i} />
          case 'label':
            return <DropdownMenuLabel key={i}>{item.label}</DropdownMenuLabel>
          case 'sub':
            return (
              <DropdownMenuSub key={i}>
                <DropdownMenuSubTrigger disabled={item.disabled}>{item.icon}{item.label}</DropdownMenuSubTrigger>
                <DropdownMenuSubContent className="max-h-[70vh] overflow-y-auto">
                  <Items items={item.items} onClose={onClose} />
                </DropdownMenuSubContent>
              </DropdownMenuSub>
            )
          default:
            return (
              <DropdownMenuItem key={i} disabled={item.disabled} variant={item.destructive ? 'destructive' : 'default'}
                onSelect={() => { onClose(); item.onSelect() }}>
                {item.icon}{item.label}
                {item.shortcut && <DropdownMenuShortcut>{item.shortcut}</DropdownMenuShortcut>}
              </DropdownMenuItem>
            )
        }
      })}
    </>
  )
}

/** Comandos comuns a qualquer canvas, expostos para atalhos e menus do Shell. */
export interface CanvasCommands {
  selectAll: () => void
  /** Cada comando devolve a mensagem para o toast, ou null se não havia o que fazer. */
  deleteSelected: () => Promise<string | null>
  copy: () => string | null
  cut: () => Promise<string | null>
  paste: () => Promise<string | null>
  duplicate: () => Promise<string | null>
  fitAll: () => void
  zoomIn: () => void
  zoomOut: () => void
}
