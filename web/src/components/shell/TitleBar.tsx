// Barra de título: menus, nome do projeto e ações rápidas.

import { Bot, Moon, PanelLeft, PanelRight, Plug, Presentation, Redo2, Sun, Undo2, Waypoints } from 'lucide-react'
import type { ReactNode } from 'react'
import { cn } from '@/lib/utils'
import { useTheme } from '../../lib/theme'
import { Button } from '../ui'
import { Button as UIButton } from '../ui/button'
import { Tooltip, TooltipContent, TooltipTrigger } from '../ui/tooltip'
import { AppMenubar, type MenuActions } from './AppMenubar'

export function TitleBar({ actions, projectName, version, panels, onPanels, executive, onExecutive, archActive, generating }: {
  actions: MenuActions
  projectName: string
  version: string
  panels: { left: boolean; right: boolean }
  onPanels: (p: { left: boolean; right: boolean }) => void
  executive: boolean
  onExecutive: (v: boolean) => void
  archActive: boolean
  generating: boolean
}) {
  const theme = useTheme()
  return (
    <header className="flex h-9 shrink-0 items-center gap-2 border-b bg-chrome px-2">
      <span className="flex size-6 items-center justify-center rounded-sm border bg-background text-foreground">
        <Waypoints size={14} />
      </span>
      <AppMenubar actions={actions} theme={theme.preference} onTheme={theme.setPreference}
        panels={panels} onPanels={onPanels} executive={executive} onExecutive={onExecutive} archActive={archActive} />

      <div className="mx-auto hidden min-w-0 items-center gap-1.5 truncate text-[12px] text-muted-foreground lg:flex">
        <span className="truncate font-medium text-foreground">{projectName}</span>
        <span>· v{version}</span>
      </div>

      <div className="ml-auto flex items-center gap-1">
        <IconButton label="Desfazer (Ctrl+Z)" onClick={() => actions.undo?.()} disabled={!actions.undo}><Undo2 /></IconButton>
        <IconButton label="Refazer (Ctrl+Y)" onClick={() => actions.redo?.()} disabled={!actions.redo}><Redo2 /></IconButton>
        <div className="mx-1 h-4 w-px bg-border" />
        <IconButton label={panels.left ? 'Ocultar Toolbox (Ctrl+B)' : 'Mostrar Toolbox (Ctrl+B)'} active={panels.left}
          onClick={() => onPanels({ ...panels, left: !panels.left })}><PanelLeft /></IconButton>
        <IconButton label={panels.right ? 'Ocultar painéis (Ctrl+J)' : 'Mostrar painéis (Ctrl+J)'} active={panels.right}
          onClick={() => onPanels({ ...panels, right: !panels.right })}><PanelRight /></IconButton>
        <IconButton label="Conectar agente de IA (MCP)" onClick={actions.mcp}><Plug /></IconButton>
        <IconButton label={theme.resolved === 'dark' ? 'Tema claro' : 'Tema escuro'} onClick={theme.toggle}>
          {theme.resolved === 'dark' ? <Sun /> : <Moon />}
        </IconButton>
        <div className="mx-1 h-4 w-px bg-border" />
        <Button size="sm" variant="secondary" icon={Presentation} onClick={actions.pitch}>Pitch</Button>
        <Button size="sm" variant="primary" icon={Bot} loading={generating} onClick={actions.generatePRD}>Gerar AI-PRD</Button>
      </div>
    </header>
  )
}

function IconButton({ label, onClick, children, active, disabled }: {
  label: string; onClick: () => void; children: ReactNode; active?: boolean; disabled?: boolean
}) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        {/* span: o tooltip continua funcionando com o botão desabilitado */}
        <span className="inline-flex">
        <UIButton size="icon-sm" variant="ghost" onClick={onClick} aria-label={label} disabled={disabled}
          className={cn(active && 'text-foreground')}>
          {children}
        </UIButton>
        </span>
      </TooltipTrigger>
      <TooltipContent>{label}</TooltipContent>
    </Tooltip>
  )
}
