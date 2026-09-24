// Abas abertas na área central (persistidas por navegador).

import { useCallback, useEffect, useMemo } from 'react'
import { useStored } from '../../lib/storage'
import { normalizeTabKey, parseTabKey, tabKey, type TabRef } from '../../lib/tabs'
import type { UMLDiagram } from '../../lib/types'

export function useWorkspaceTabs(diagrams: UMLDiagram[]) {
  const [tabKeys, setTabKeys] = useStored<string[]>('archcode-tabs', ['arch'])
  const [activeKey, setActiveKey] = useStored<string>('archcode-active-tab', 'arch')

  // Abas gravadas por versões anteriores (ex.: "view:docs", a antiga aba única de
  // Documentação) são convertidas uma vez para o formato atual.
  useEffect(() => {
    const keys = [...new Set(tabKeys.map(normalizeTabKey).filter((k): k is string => !!k))]
    if (keys.join('|') !== tabKeys.join('|')) setTabKeys(keys)
    const active = normalizeTabKey(activeKey) ?? keys[0] ?? ''
    if (active !== activeKey) setActiveKey(active)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  // Abas cujo diagrama deixou de existir (excluído por IA ou no disco) somem.
  const tabs = useMemo(() => tabKeys
    .map(parseTabKey)
    .filter((t): t is TabRef => !!t && (t.type !== 'uml' || diagrams.some((d) => d.id === t.id))),
  [tabKeys, diagrams])
  const active = tabs.find((t) => tabKey(t) === activeKey) ?? tabs[0] ?? null
  const activeDiagram = active?.type === 'uml' ? diagrams.find((d) => d.id === active.id) ?? null : null

  const open = useCallback((tab: TabRef) => {
    const key = tabKey(tab)
    if (!tabKeys.includes(key)) setTabKeys([...tabKeys, key])
    setActiveKey(key)
  }, [setActiveKey, setTabKeys, tabKeys])

  const close = useCallback((key: string) => {
    const idx = tabKeys.indexOf(key)
    const next = tabKeys.filter((k) => k !== key)
    setTabKeys(next)
    if (activeKey === key) setActiveKey(next[Math.max(0, idx - 1)] ?? '')
  }, [activeKey, setActiveKey, setTabKeys, tabKeys])

  const activate = useCallback((tab: TabRef) => setActiveKey(tabKey(tab)), [setActiveKey])

  return { tabs, active, activeKey, activeDiagram, open, close, activate }
}
