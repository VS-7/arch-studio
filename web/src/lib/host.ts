// Onde a interface está rodando: no navegador (servidor `archcode-studio serve`)
// ou dentro do app desktop, que marca o index.html com
// <meta name="archcode-host" content="desktop">.

export type Host = 'web' | 'desktop'

export const HOST: Host =
  document.querySelector('meta[name="archcode-host"]')?.getAttribute('content') === 'desktop' ? 'desktop' : 'web'
