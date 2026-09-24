const SYMBOLS: Record<string, string> = { BRL: 'R$', USD: 'US$', EUR: '€', GBP: '£' }

const LOCALES: Record<string, string> = { BRL: 'pt-BR', USD: 'en-US', EUR: 'de-DE', GBP: 'en-GB' }

export function money(currency: string, value: number): string {
  const locale = LOCALES[currency] ?? 'pt-BR'
  const symbol = SYMBOLS[currency] ?? `${currency} `
  return `${symbol} ${value.toLocaleString(locale, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`
}

export function hours(value: number): string {
  return Number.isInteger(value) ? `${value}h` : `${value.toFixed(1)}h`
}

export function percent(value: number): string {
  return `${Math.round(value)}%`
}

export function compactNumber(value: number): string {
  return value.toLocaleString('pt-BR', { maximumFractionDigits: 1 })
}

export function relativeTime(iso: string): string {
  const then = new Date(iso).getTime()
  if (Number.isNaN(then)) return ''
  const seconds = Math.round((Date.now() - then) / 1000)
  if (seconds < 10) return 'agora'
  if (seconds < 60) return `há ${seconds}s`
  const minutes = Math.round(seconds / 60)
  if (minutes < 60) return `há ${minutes}min`
  const hoursAgo = Math.round(minutes / 60)
  if (hoursAgo < 24) return `há ${hoursAgo}h`
  return new Date(iso).toLocaleDateString('pt-BR')
}

/** Nome de arquivo seguro a partir de um título ("Projeto Ágil" → "projeto-agil"). */
export function slugify(value: string, fallback = 'arquivo'): string {
  return value.toLowerCase().normalize('NFD').replace(/[̀-ͯ]/g, '')
    .replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '') || fallback
}
