// Layout dos painéis (preferências por navegador): Toolbox e barra lateral
// visíveis, larguras, proporção Model Explorer/Editor e seções recolhidas.

import { useEffect, useState } from 'react'
import { useStored } from '../../lib/storage'

export const LEFT_DEFAULT = 220
export const LEFT_MIN = 160
export const RIGHT_DEFAULT = 310
export const RIGHT_MIN = 220
export const EXPLORER_DEFAULT = 0.45

export type SideSection = 'explorer' | 'editor'
/** Seções da barra lateral direita: abertas/recolhidas e qual está maximizada. */
type SideSections = { explorer: boolean; editor: boolean; max: SideSection | null }
const SECTIONS_DEFAULT: SideSections = { explorer: true, editor: true, max: null }

const clamp = (v: number, min: number, max: number) => Math.min(max, Math.max(min, v))

/** Largura da janela, para limitar os painéis laterais em telas menores. */
function useViewportWidth(): number {
  const [width, setWidth] = useState(() => window.innerWidth)
  useEffect(() => {
    const onResize = () => setWidth(window.innerWidth)
    window.addEventListener('resize', onResize)
    return () => window.removeEventListener('resize', onResize)
  }, [])
  return width
}

export function useWorkspaceLayout() {
  const [panels, setPanels] = useStored('archcode-panels', { left: true, right: true })
  const [leftWidth, setLeftWidth] = useStored('archcode-left-width', LEFT_DEFAULT)
  const [rightWidth, setRightWidth] = useStored('archcode-right-width', RIGHT_DEFAULT)
  const [explorerRatio, setExplorerRatio] = useStored('archcode-explorer-ratio', EXPLORER_DEFAULT)
  const [sections, setSections] = useStored<SideSections>('archcode-side-sections', SECTIONS_DEFAULT)
  const viewport = useViewportWidth()

  // Larguras limitadas pela janela: a área central nunca fica espremida.
  const leftMax = Math.max(LEFT_MIN, Math.min(520, viewport * 0.35))
  const rightMax = Math.max(RIGHT_MIN, Math.min(760, viewport * 0.5))

  // Com uma seção maximizada, a outra mostra só o cabeçalho.
  const explorerOpen = sections.max ? sections.max === 'explorer' : sections.explorer
  const editorOpen = sections.max ? sections.max === 'editor' : sections.editor

  return {
    panels,
    setPanels,
    togglePanel: (side: 'left' | 'right') => setPanels({ ...panels, [side]: !panels[side] }),
    left: { width: clamp(leftWidth, LEFT_MIN, leftMax), max: leftMax, setWidth: setLeftWidth },
    right: { width: clamp(rightWidth, RIGHT_MIN, rightMax), max: rightMax, setWidth: setRightWidth },
    explorerRatio,
    setExplorerRatio,
    explorerOpen,
    editorOpen,
    maximized: sections.max,
    toggleSection: (section: SideSection) => {
      if (!sections.max) setSections({ ...sections, [section]: !sections[section] })
      // Recolher a seção maximizada devolve o espaço à outra; expandir a outra restaura as duas.
      else if (sections.max === section) setSections({ explorer: section !== 'explorer', editor: section !== 'editor', max: null })
      else setSections(SECTIONS_DEFAULT)
    },
    maximizeSection: (section: SideSection) => {
      setSections(sections.max === section ? { ...sections, max: null } : { ...sections, [section]: true, max: section })
    },
    /** Garante o Editor visível (ex.: F2 / duplo clique para renomear). */
    revealEditor: () => {
      if (!panels.right) setPanels({ ...panels, right: true })
      if (!editorOpen) setSections({ ...sections, editor: true, max: sections.max === 'explorer' ? null : sections.max })
    },
    resetLayout: () => {
      setPanels({ left: true, right: true })
      setLeftWidth(LEFT_DEFAULT)
      setRightWidth(RIGHT_DEFAULT)
      setExplorerRatio(EXPLORER_DEFAULT)
      setSections(SECTIONS_DEFAULT)
    },
  }
}

export type WorkspaceLayout = ReturnType<typeof useWorkspaceLayout>
