// Isola falhas de renderização: um erro em uma tela (ex.: dado inesperado do
// servidor) mostra um aviso no lugar dela em vez de desmontar o app inteiro.

import { AlertTriangle } from 'lucide-react'
import { Component, type ErrorInfo, type ReactNode } from 'react'
import { Button } from './button'

interface Props {
  children: ReactNode
  /** Nome da área, exibido na mensagem ("Precificação", "Editor"…). */
  area?: string
}

interface State { error: Error | null }

export class ErrorBoundary extends Component<Props, State> {
  state: State = { error: null }

  static getDerivedStateFromError(error: Error): State {
    return { error }
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error(`[ArchCode] falha ao renderizar ${this.props.area ?? 'a tela'}:`, error, info.componentStack)
  }

  render() {
    const { error } = this.state
    if (!error) return this.props.children
    return (
      <div className="grid h-full place-items-center p-6">
        <div className="max-w-md text-center">
          <AlertTriangle size={26} className="mx-auto text-warning" />
          <p className="mt-2 text-sm font-semibold">
            Não foi possível exibir {this.props.area ? `“${this.props.area}”` : 'esta tela'}
          </p>
          <p className="mt-1 break-words font-mono text-[11.5px] text-muted-foreground">{error.message}</p>
          <p className="mt-2 text-xs text-muted-foreground">O restante do Studio continua funcionando.</p>
          <Button size="sm" variant="outline" className="mt-3" onClick={() => this.setState({ error: null })}>
            Tentar novamente
          </Button>
        </div>
      </div>
    )
  }
}
