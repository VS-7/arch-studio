---
name: estilo-typescript
description: Mantém TypeScript e React rigorosos — strict ligado, typecheck e lint limpos, sem any injustificado, regras dos hooks, componentes pequenos e markup acessível. Vale sempre que a tarefa tocar arquivos .ts ou .tsx.
category: padronizacao
version: 1.0.0
trigger: sempre
applies_to:
  stacks: [typescript, react]
checks:
  - id: typecheck-limpo
    text: O typecheck do projeto termina sem erros.
    command: npx tsc --noEmit
    required: true
  - id: strict-ligado
    text: A configuração efetiva do TypeScript mantém strict ativado e nenhuma opção de rigor desligada.
    command: npx tsc --showConfig
    required: true
  - id: lint-limpo
    text: O lint configurado no projeto termina sem erros.
    command: npm run lint
    required: true
  - id: sem-any-injustificado
    text: Nenhum any, as any ou @ts-ignore novo sem comentário justificando.
    required: false
  - id: regras-dos-hooks
    text: Hooks são chamados só no topo de componentes ou hooks customizados, sem supressão da regra exhaustive-deps.
    required: false
  - id: markup-acessivel
    text: Todo controle interativo novo é elemento semântico com rótulo acessível (label, aria-label ou texto visível).
    required: false
  - id: sem-export-morto
    text: Nenhum export novo fica sem uso no projeto.
    required: false
---

# Estilo TypeScript e React

## Quando usar

Sempre que a tarefa criar ou alterar arquivos `.ts` ou `.tsx`. Rode os comandos na pasta do pacote que contém o `tsconfig.json` (ex.: `web/`). A configuração existente de lint e formatação manda; estas regras cobrem o resto.

## Regras

1. **strict é inegociável.** Não desligue `strict`, `noImplicitAny` ou `strictNullChecks` para "passar" — corrija o tipo.
2. **Sem `any` gratuito.** Prefira `unknown` com estreitamento, genéricos ou tipos derivados (`z.infer`, `ReturnType`). `any`, `as any` e `@ts-ignore` só com comentário do porquê; prefira `@ts-expect-error` com motivo.
   ```ts
   // ❌ const data: any = await res.json()
   const data: unknown = await res.json()
   if (!isPedido(data)) throw new Error('resposta inválida de /api/v1/orders')
   ```
3. **Tipos na fronteira.** Respostas de API são tipadas e validadas num só lugar (o cliente da API), seguindo `api/endpoints.yaml`; componentes recebem tipos prontos.
4. **Regras dos hooks.** Hooks só no topo do componente ou de hook customizado, nunca em condição ou laço. Dependências completas; não silencie `react-hooks/exhaustive-deps` — reestruture (`useCallback`, mover a função para dentro do efeito). Estado derivado se calcula no render ou com `useMemo`, não em `useEffect`.
5. **Componentes pequenos.** Uma responsabilidade por componente; passou de ~150 linhas ou acumulou vários `useState` relacionados, extraia subcomponente ou hook. Props com tipo nomeado.
6. **Acessibilidade.** `<button>` para ação e `<a href>` para navegação — nunca `div onClick`. Todo input com `<label htmlFor>` ou `aria-label`; botão só com ícone tem `aria-label`; imagem tem `alt`. Reuse os componentes acessíveis já adotados (ex.: shadcn/ui, Radix).
7. **Sem código morto.** Nada de export, prop ou arquivo sem uso; remova `console.log` de depuração.
8. **Siga o existente.** Imports, aspas, ponto e vírgula e organização de pastas como no código ao redor e na config de lint/formatter (ESLint, Prettier, Biome).

## Como verificar

- `typecheck-limpo`: `npx tsc --noEmit` termina com código 0; em projetos com `references` (template Vite), use `npx tsc -b`.
- `strict-ligado`: a saída de `npx tsc --showConfig` tem `"strict": true` e nenhuma opção de rigor em `false`.
- `lint-limpo`: `npm run lint` termina com código 0 (`na` se o projeto não define script de lint).
- `sem-any-injustificado`: cada `any`, `as any` e `@ts-ignore` novo no diff tem comentário.
- `regras-dos-hooks`: nenhum `eslint-disable` de regra `react-hooks` novo no diff.
- `markup-acessivel`: nenhum `onClick` novo em `div`/`span`; inputs novos têm rótulo.
- `sem-export-morto`: cada export novo tem pelo menos um import no projeto.

## Exemplos

```tsx
// ❌
<div onClick={salvar}><SaveIcon /></div>
// ✅
<button type="button" onClick={salvar} aria-label="Salvar pedido"><SaveIcon aria-hidden /></button>
```
