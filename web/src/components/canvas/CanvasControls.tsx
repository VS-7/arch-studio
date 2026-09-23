// Controles de zoom do canvas (substituem o <Controls> do React Flow) com
// tooltips do design system, no canto inferior esquerdo como no StarUML.

import { Panel, useReactFlow } from '@xyflow/react'
import { Maximize, Minus, Plus } from 'lucide-react'
import { IconAction } from '../ui'

export function CanvasControls() {
  const flow = useReactFlow()
  return (
    <Panel position="bottom-left" className="flex flex-col overflow-hidden rounded-sm border bg-card shadow-xs">
      <IconAction label="Aproximar (+)" side="right" className="size-6 rounded-none"
        onClick={() => void flow.zoomIn({ duration: 200 })}><Plus size={13} /></IconAction>
      <IconAction label="Afastar (−)" side="right" className="size-6 rounded-none border-y"
        onClick={() => void flow.zoomOut({ duration: 200 })}><Minus size={13} /></IconAction>
      <IconAction label="Ajustar à tela (Shift+1)" side="right" className="size-6 rounded-none"
        onClick={() => void flow.fitView({ padding: 0.15, duration: 400, maxZoom: 1.2 })}><Maximize size={12} /></IconAction>
    </Panel>
  )
}
