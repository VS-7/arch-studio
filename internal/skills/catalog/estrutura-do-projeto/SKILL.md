---
name: estrutura-do-projeto
description: Mantém o layout de pastas espelhando a arquitetura — cada componente do canvas é um módulo com o seu nome, tiers viram camadas e as mudanças da tarefa ficam na pasta do seu componente. Use ao iniciar qualquer tarefa, antes de criar arquivos.
category: organizacao
version: 1.0.0
trigger: ao_iniciar_tarefa
checks:
  - id: mudancas-no-componente
    text: Todo arquivo alterado pela tarefa está na pasta do componente da tarefa, num pacote compartilhado explícito ou é arquivo de raiz necessário (manifesto, lockfile, .env.example).
    command: git diff --name-only main...HEAD
    required: true
  - id: pasta-com-nome-do-componente
    text: Módulo ou pasta nova de componente tem nome derivado do rótulo do nó no diagrama.
    required: false
  - id: camadas-respeitadas
    text: Código do tier domain não importa handlers, clientes de rede, drivers de banco nem código de interface.
    required: true
  - id: compartilhado-explicito
    text: Código usado por dois ou mais componentes está num pacote compartilhado com nome de assunto, não duplicado.
    required: false
  - id: raiz-sem-pasta-nova-sem-adr
    text: Nenhuma pasta de primeiro nível nova foi criada sem ADR registrada com upsert_adr.
    required: true
---

# Estrutura do projeto

## Quando usar

Ao iniciar qualquer tarefa, antes de criar o primeiro arquivo. O diagrama (`get_system_context`) diz quais componentes existem e em que tier cada um está; a árvore de pastas deve permitir achar o código de um componente só pelo nome dele no canvas.

## Regras

1. **Um componente, uma pasta.** O rótulo do nó vira o nome do módulo, na convenção da linguagem: `Core API` → `services/core-api/` (monorepo) ou `internal/coreapi/` (módulo Go único); `Web App` → `apps/web-app/`. Descubra o padrão já usado no repositório e siga-o; só defina um novo se não houver nenhum — e registre com `upsert_adr`.
2. **Tiers viram camadas dentro do componente.**

   | Tier | O que contém | Pode depender de |
   | --- | --- | --- |
   | `data` | migrações, schemas, repositórios | `domain` |
   | `domain` | entidades, regras, portas (interfaces) | nada de IO |
   | `backend` | handlers, casos de uso, serviços | `domain`, `data` |
   | `integration` | clientes de APIs externas, filas, webhooks | `domain` |
   | `frontend` | telas, componentes, estado de UI | contratos da API |
   | `devops` | `deploy/`, `.github/workflows/`, `Dockerfile`, IaC | — |

3. **A tarefa fica no seu componente.** O arquivo da tarefa (`.arch/plan/tasks/<ID>.md`) indica o componente. Mexer em outro componente é sinal de tarefa mal cortada: registre `create_backlog_item` em vez de ampliar o escopo; se for inevitável, explique no PR.
4. **Compartilhado é explícito.** Código usado por 2+ componentes vai para um pacote com nome de assunto (`internal/shared/money`, `packages/shared-auth`) — nunca `utils`, `common` ou `helpers` genéricos. Não copie código entre componentes.
5. **Pasta nova na raiz exige ADR.** Criar `libs/`, `tools/`, `infra/` etc. muda a estrutura para todo o time: registre `upsert_adr` (contexto, decisão, consequências) antes.
6. **Artefatos do Studio têm lugar fixo.** `.arch/`, `docs/` e `api/` são gerenciados pelas ferramentas; não coloque código neles nem mova seus arquivos.
7. **Testes ao lado do código** (`pedido_test.go`, `Pedido.test.tsx`) ou na convenção já usada no componente.

## Como verificar

- `mudancas-no-componente`: `git diff --name-only main...HEAD` (troque `main` pela branch base) lista só caminhos do componente, do pacote compartilhado ou arquivos de raiz necessários.
- `pasta-com-nome-do-componente`: o nome da pasta nova é reconhecível a partir do rótulo do nó.
- `camadas-respeitadas`: os imports dos arquivos de `domain` no diff não citam handler, cliente HTTP, driver de banco nem UI.
- `compartilhado-explicito`: nenhum bloco duplicado entre componentes no diff.
- `raiz-sem-pasta-nova-sem-adr`: se o diff cria pasta de primeiro nível, há ADR correspondente em `docs/architecture-decisions/`.

## Exemplos

- ❌ `utils/format.go` usado por `core-api` e `billing` → ✅ `internal/shared/money/format.go`.
- ❌ Tarefa do `billing` alterando `services/core-api/handlers/user.go` "de passagem" → ✅ `create_backlog_item` com `type: task` para o `core-api`.
- ❌ `domain/pedido.go` importando `database/sql` → ✅ `domain` declara a porta `PedidoRepo`; `data` a implementa.
