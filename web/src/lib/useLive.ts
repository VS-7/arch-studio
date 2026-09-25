// Dados que não estão no snapshot (retomada, Git, skills, sessões…): carrega
// uma vez e recarrega quando o servidor anuncia um dos eventos informados.

import { useCallback, useEffect, useRef, useState } from 'react'
import { errorMessage } from './errors'
import { useProject } from './project'

export function useLive<T>(load: () => Promise<T>, events: string[], deps: unknown[] = []) {
  const { lastEvent } = useProject()
  const [data, setData] = useState<T | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const loadRef = useRef(load)
  loadRef.current = load

  const reload = useCallback(async () => {
    try {
      setData(await loadRef.current())
      setError(null)
    } catch (err) {
      setError(errorMessage(err))
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { void reload() }, [reload, ...deps])

  const watched = events.join('|')
  useEffect(() => {
    if (lastEvent && watched.split('|').includes(lastEvent.type)) void reload()
  }, [lastEvent, watched, reload])

  return { data, error, loading, reload }
}
