// Erros vindos do servidor e a mensagem que o usuário vê.

export class ApiError extends Error {
  constructor(
    message: string,
    readonly status: number,
    readonly hint?: string,
  ) {
    super(message)
    this.name = 'ApiError'
  }
}

/** Texto para toasts: a mensagem e, quando o servidor sugere, a dica de correção. */
export function errorMessage(err: unknown): string {
  if (err instanceof ApiError && err.hint) return `${err.message} — ${err.hint}`
  return err instanceof Error ? err.message : String(err)
}
