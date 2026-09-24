// Identidade visual do Documento de Requisitos, compartilhada pela
// pré-visualização/PDF (CSS em html.ts) e pelo DOCX (docx.ts): uma única fonte
// de verdade para fonte, corpo, cores e margens, para que os três formatos
// saiam iguais.
//
// Padrão de mercado para especificações: Arial 11 pt com entrelinha 1,4,
// títulos numerados em azul-marinho (a paleta "Heading" clássica do Word),
// tabelas com cabeçalho sombreado, capa com faixa de identidade e as demais
// páginas com cabeçalho e rodapé numerado.

export const THEME = {
  /** Nome da fonte no DOCX e primeira opção da pilha CSS. */
  font: 'Arial',
  fontStack: "Arial, 'Liberation Sans', Helvetica, sans-serif",
  monoStack: "'Courier New', 'Liberation Mono', monospace",
  lineHeight: 1.4,

  /** Tamanhos em pontos. */
  size: {
    body: 11,
    small: 9.5,
    table: 10,
    running: 8.5,
    h1: 18,
    h2: 14,
    h3: 12,
    h4: 11,
    plainTitle: 14,
    coverTitle: 28,
    coverProject: 18,
    coverMeta: 12,
    coverBand: 9,
  },

  /** Cores em hexadecimal sem "#" (o DOCX usa assim; o CSS prefixa). */
  color: {
    primary: '1F3864',
    accent: '2F5496',
    text: '1A1A1A',
    muted: '595959',
    rule: 'BFBFBF',
    tableHead: 'D9E2F3',
    tableBorder: 'A6A6A6',
    band: '1F3864',
    bandText: 'FFFFFF',
  },

  /** Página A4 e margens, em milímetros. */
  page: {
    width: 210,
    height: 297,
    /** Margens das páginas de conteúdo (capa: ver `cover`). */
    top: 25,
    bottom: 20,
    left: 25,
    right: 20,
    /** Distância do cabeçalho e do rodapé até a borda da folha. */
    header: 12,
    footer: 10,
    /** Capa: faixa superior, faixa inferior e recuo lateral do texto. */
    cover: { band: 16, foot: 6, inset: 30 },
  },
} as const

export type Theme = typeof THEME

/** Milímetros → pixels CSS (96 dpi). */
export const mmToPx = (mm: number): number => (mm / 25.4) * 96

/** Milímetros → twips (1/1440 de polegada), unidade do DOCX. */
export const mmToTwip = (mm: number): number => Math.round((mm / 25.4) * 1440)

/** Área útil (largura × altura) das páginas de conteúdo, em milímetros. */
export const BODY_MM = {
  width: THEME.page.width - THEME.page.left - THEME.page.right,
  height: THEME.page.height - THEME.page.top - THEME.page.bottom,
}
