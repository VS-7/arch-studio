// Paleta de componentes (RF005). Arraste para o canvas ou clique para criar.

import { useState } from 'react'
import { NODE_META, NODE_TYPES } from '../../lib/nodeMeta'
import { api } from '../../lib/api'
import type { NodeType } from '../../lib/types'
import { Button, Input, useToast } from '../ui'

const GROUPS: { title: string; types: NodeType[] }[] = [
  { title: 'Computação', types: ['compute', 'queue'] },
  { title: 'Dados', types: ['database', 'cache', 'storage'] },
  { title: 'Borda & Integração', types: ['gateway', 'external_service'] },
  { title: 'Clientes', types: ['client'] },
  { title: 'Organização', types: ['group'] },
]

export function Palette({ onCreated }: { onCreated: (id: string) => void }) {
  const toast = useToast()
  const [pending, setPending] = useState<NodeType | null>(null)
  const [label, setLabel] = useState('')

  const create = async (type: NodeType) => {
    const name = label.trim()
    if (!name) return
    try {
      const node = await api.addNode({ label: name, type, tier: NODE_META[type].tier })
      toast('success', `Componente "${node.data.label}" criado`)
      onCreated(node.id)
      setPending(null)
      setLabel('')
    } catch (err) {
      toast('error', (err as Error).message)
    }
  }

  return (
    <div className="flex h-full flex-col">
      <div className="border-b border-app px-3.5 py-2.5">
        <h2 className="text-[11px] font-bold uppercase tracking-wider text-muted-app">Componentes</h2>
        <p className="mt-0.5 text-[11px] leading-snug text-muted-app">Arraste para o canvas ou clique para nomear.</p>
      </div>

      <div className="flex-1 overflow-y-auto px-2.5 py-2.5">
        {GROUPS.map((group) => (
          <div key={group.title} className="mb-3">
            <p className="mb-1.5 px-1 text-[10px] font-bold uppercase tracking-wider text-muted-app/70">{group.title}</p>
            <div className="space-y-1">
              {group.types.filter((t) => NODE_TYPES.includes(t)).map((type) => {
                const meta = NODE_META[type]
                const Icon = meta.icon
                const active = pending === type
                return (
                  <div key={type}>
                    <button
                      draggable
                      onDragStart={(e) => {
                        e.dataTransfer.setData('application/archcode-node', JSON.stringify({ type }))
                        e.dataTransfer.effectAllowed = 'copy'
                      }}
                      onClick={() => { setPending(active ? null : type); setLabel('') }}
                      title={meta.hint}
                      className="flex w-full cursor-grab items-center gap-2.5 rounded-lg border border-transparent px-2 py-1.5 text-left transition-colors hover:surface-3 active:cursor-grabbing"
                      style={active ? { borderColor: meta.color, backgroundColor: `${meta.color}12` } : undefined}
                    >
                      <span className="flex h-7 w-7 shrink-0 items-center justify-center rounded-md"
                        style={{ backgroundColor: `${meta.color}1f`, color: meta.color }}>
                        <Icon size={14} />
                      </span>
                      <span className="min-w-0 flex-1">
                        <span className="block truncate text-[12.5px] font-medium text-app">{meta.label}</span>
                      </span>
                    </button>
                    {active && (
                      <div className="mt-1.5 space-y-1.5 rounded-lg surface-3 p-2">
                        <Input autoFocus value={label} placeholder={`Nome do ${meta.label.toLowerCase()}`}
                          onChange={(e) => setLabel(e.target.value)}
                          onKeyDown={(e) => {
                            if (e.key === 'Enter') void create(type)
                            if (e.key === 'Escape') setPending(null)
                          }} />
                        <div className="flex gap-1.5">
                          <Button size="sm" variant="primary" onClick={() => void create(type)} disabled={!label.trim()}>
                            Criar
                          </Button>
                          <Button size="sm" variant="ghost" onClick={() => setPending(null)}>Cancelar</Button>
                        </div>
                      </div>
                    )}
                  </div>
                )
              })}
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}
