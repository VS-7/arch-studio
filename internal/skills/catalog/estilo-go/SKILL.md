---
name: estilo-go
description: Mantém o código Go idiomático e verificável — gofmt e go vet limpos, erros checados e embrulhados com %w, sem panic em biblioteca, context propagado, interfaces pequenas no consumidor e testes em tabela. Vale sempre que a tarefa tocar arquivos .go.
category: padronizacao
version: 1.0.0
trigger: sempre
applies_to:
  stacks: [go]
checks:
  - id: gofmt-limpo
    text: O gofmt não lista nenhum arquivo fora do padrão.
    command: gofmt -l .
    required: true
  - id: go-vet-limpo
    text: O go vet termina sem apontamentos.
    command: go vet ./...
    required: true
  - id: testes-go-passam
    text: Todos os testes do módulo passam.
    command: go test ./...
    required: true
  - id: erros-tratados
    text: Nenhum erro novo é descartado sem comentário, e erros propagados com contexto usam fmt.Errorf com %w.
    required: true
  - id: sem-panic-em-biblioteca
    text: Nenhum panic, log.Fatal ou os.Exit novo fora de main e de testes.
    required: false
  - id: context-propagado
    text: Funções novas que fazem IO recebem context.Context como primeiro parâmetro e o repassam.
    required: false
  - id: sem-estado-global-mutavel
    text: Nenhuma variável de pacote mutável nova; dependências entram por construtor.
    required: false
  - id: staticcheck-limpo
    text: O staticcheck não aponta problemas.
    command: staticcheck ./...
    required: false
---

# Estilo Go

## Quando usar

Sempre que a tarefa criar ou alterar arquivos `.go`. Siga primeiro o estilo do código ao redor; estas regras decidem o que o código existente não decide.

## Regras

1. **Formatação e vet não se discutem.** `gofmt -w` (ou `goimports -w`) nos arquivos tocados e `go vet ./...` antes de cada commit.
2. **Todo erro é checado.** Nada de `_ = f()` para esconder erro; se ignorar é correto, comente por quê. Embrulhe com contexto e `%w`; compare com `errors.Is`/`errors.As`, nunca pelo texto. Mensagens em minúsculas, sem ponto final, sem "erro ao" redundante.
   ```go
   if err := repo.Save(ctx, p); err != nil {
   	return fmt.Errorf("salvar pedido %s: %w", p.ID, err)
   }
   ```
3. **Sem panic em biblioteca.** Retorne `error`. `panic` só para invariante de programação impossível; `log.Fatal` e `os.Exit` só em `main`.
4. **Context primeiro.** Funções com IO recebem `ctx context.Context` como primeiro parâmetro e o repassam (`QueryContext`, `http.NewRequestWithContext`). Não guarde `ctx` em struct; não crie `context.Background()` no meio do fluxo.
5. **Interfaces pequenas, no consumidor.** Declare a interface no pacote que a usa, só com os métodos de que ele precisa; o produtor retorna tipos concretos.
   ```go
   // no pacote que consome
   type pedidoSalvador interface {
   	Save(ctx context.Context, p Pedido) error
   }
   ```
6. **Sem estado global mutável.** DB, clientes, logger e relógio entram por construtor; nada de `var db *sql.DB` de pacote. Constantes e tabelas somente leitura são ok.
7. **Testes em tabela** com `t.Run` e nomes descritivos; `t.Helper()` em helpers, `t.TempDir()` para arquivos, `t.Parallel()` quando seguro.
   ```go
   casos := []struct {
   	nome string
   	in   string
   	quer int
   }{
   	{"vazio", "", 0},
   	{"um item", "a", 1},
   }
   for _, tc := range casos {
   	t.Run(tc.nome, func(t *testing.T) { /* ... */ })
   }
   ```
8. **Concorrência com dono.** Toda goroutine tem forma de terminar (ctx ou canal); use `errgroup` para grupos; rode `go test -race ./...` ao mexer em concorrência.
9. **Comentários no idioma do projeto**, iguais ao código ao redor; doc de identificador exportado começa pelo nome dele.
10. **Nomes curtos e claros.** `ctx`, `err`, `ok`; receptor de 1–2 letras; sem gagueira de pacote (`pedido.Service`, não `pedido.PedidoService`).

## Como verificar

- `gofmt-limpo`: `gofmt -l .` produz saída vazia.
- `go-vet-limpo`: `go vet ./...` termina com código 0.
- `testes-go-passam`: `go test ./...` termina com código 0.
- `erros-tratados`: revise no diff cada `_ =` e cada `return err` sem contexto em fronteira de pacote.
- `sem-panic-em-biblioteca`: `panic(`, `log.Fatal` e `os.Exit` novos no diff aparecem só em `main` ou em testes.
- `context-propagado`: funções novas com IO têm `ctx context.Context` como primeiro parâmetro.
- `sem-estado-global-mutavel`: nenhum `var` de pacote novo reatribuído fora da declaração.
- `staticcheck-limpo`: `staticcheck ./...` termina com código 0 (`na` se a ferramenta não estiver instalada, dito na evidência).
