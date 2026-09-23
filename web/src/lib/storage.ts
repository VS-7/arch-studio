// Preferências de layout persistidas por navegador (larguras, painéis, zoom…).

import { useCallback, useState } from 'react'

export function useStored<T>(key: string, initial: T): [T, (v: T) => void] {
  const [value, setValue] = useState<T>(() => {
    try {
      const raw = localStorage.getItem(key)
      return raw ? (JSON.parse(raw) as T) : initial
    } catch { return initial }
  })
  const set = useCallback((v: T) => {
    setValue(v)
    try { localStorage.setItem(key, JSON.stringify(v)) } catch { /* ignora */ }
  }, [key])
  return [value, set]
}
