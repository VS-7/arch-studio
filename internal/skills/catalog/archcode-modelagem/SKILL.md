---
name: archcode-modelagem
description: Modela uma funcionalidade nova antes de qualquer código — contexto, requisito, caso de uso com critérios Given-When-Then, só os componentes e conexões necessários com metadados completos, validação sem erros e backlog sincronizado. Use sempre que surgir funcionalidade ou conexão fora do diagrama.
category: studio
version: 1.0.0
scope: modelagem
trigger: sempre
checks:
  - id: contexto-lido
    text: get_system_context foi chamado antes de qualquer alteração no modelo.
    required: true
  - id: requisito-registrado
    text: A funcionalidade tem requisito funcional registrado com upsert_requirement e ligado aos componentes.
    required: true
  - id: caso-de-uso-completo
    text: O caso de uso registrado com upsert_use_case tem fluxo principal, exceções e critérios de aceite Given-When-Then.
    required: true
  - id: nos-minimos-ancorados
    text: Só foram criados os componentes indispensáveis, cada um com tipo, tecnologia, tier e connect_to para um nó existente.
    required: false
  - id: conexoes-com-metadados
    text: Toda conexão nova tem protocolo, porta, segurança e, quando HTTP, endpoints.
    required: true
  - id: regras-sem-erro
    text: A validação das regras de arquitetura não retorna erros.
    command: archcode-studio validate
    required: true
  - id: backlog-sincronizado
    text: sync_backlog foi executado depois da modelagem e o backlog contém as tarefas da funcionalidade.
    required: true
  - id: relato-curto
    text: O relato final ao humano tem no máximo 5 linhas.
    required: false
---

# Modelagem no ArchCode

## Quando usar

Sempre que surgir funcionalidade nova ou quando uma tarefa precisar de componente ou conexão que não existe no diagrama. Modelar vem antes de codar: o modelo gera requisitos, casos de uso, contratos e o backlog que depois será implementado. Este é o roteiro do prompt `model_new_feature`.

## Regras

Siga os passos na ordem, sem pular nenhum.

1. **Contexto** — `get_system_context`. Identifique componentes, tiers e conexões que já resolvem parte do pedido. Reuse antes de criar.
2. **Requisito** — `upsert_requirement`: `type: "RF"`, `title` curto, `description` no formato "O sistema deve…", `priority`, `components` com os ids envolvidos. Restrição de desempenho ou segurança vira `RNF` com `category`.
3. **Caso de uso** — `upsert_use_case`: `name` com verbo no infinitivo, `actors`, `requirements`, `components`, `main_flow` em passos, `exceptions` (entrada inválida, sem permissão, dependência fora do ar) e `acceptance` em Given-When-Then:
   `"Dado um usuário autenticado, quando solicita o boleto de um pedido já pago, então recebe 409 com a mensagem 'pedido já pago'"`
4. **Componentes** — `add_architecture_node` só para o que falta: `label`, `type`, `technology`, `tier`, `description` e `connect_to` (id ou rótulo do nó existente que ancora a posição) com `protocol`. Nunca reposicione nós existentes.
5. **Conexões** — `connect_nodes` com `source_id`, `target_id`, `protocol`, `port`, `security` (`JWT`, `mTLS`, `TLS`, `API Key`…), `description` e, para HTTP, `endpoints` (`method`, `path`, `summary`, `auth`, `request`, `response`, `status_codes`). Cliente nunca se conecta a banco.
6. **Validação** — `validate_architecture_rules`. Corrija todos os erros (ex.: `R010-missing-authentication`, `R008-edge-without-protocol`, `R004-untraced-component`) e reavalie os avisos. Repita até zero erros.
7. **Backlog** — `sync_backlog`: o backlog recebe épicos, histórias e tarefas derivados do modelo, com critérios de aceite.
8. **Relato em até 5 linhas** — o que foi modelado (RF, CDU, nós, arestas), resultado da validação, tarefas criadas e o que depende de confirmação humana.

Decisão com trade-off relevante (tecnologia nova, banco novo, síncrono ou fila) → `upsert_adr` antes do passo 4.

Nunca edite `.arch/diagrams/*.json`, `docs/requisitos.md` ou `docs/casos-de-uso/` à mão: as ferramentas mantêm IDs, rastreabilidade e posições consistentes.

## Como verificar

- `contexto-lido`: `get_system_context` precede a primeira chamada que altera o modelo.
- `requisito-registrado`: o RF existe e lista os `components` da funcionalidade.
- `caso-de-uso-completo`: o CDU tem `main_flow`, `exceptions` e ao menos um item em `acceptance` no formato Dado/Quando/Então.
- `nos-minimos-ancorados`: cada nó novo foi criado com `connect_to` e é necessário para algum passo do fluxo principal.
- `conexoes-com-metadados`: cada aresta nova tem `protocol`, `port` e `security`; as HTTP têm `endpoints`.
- `regras-sem-erro`: `validate_architecture_rules` sem erros, confirmado por `archcode-studio validate` com código 0.
- `backlog-sincronizado`: `list_backlog` mostra as tarefas geradas para o RF/CDU.
- `relato-curto`: a mensagem final tem 5 linhas ou menos.

## Exemplo de relato

```
Modelado RF012 + CDU007 "Emitir boleto" (3 critérios GWT).
Novo nó Billing Service (Go, backend) ← Core API (gRPC 50051, mTLS); Billing → Gateway de Pagamento (REST 443, API Key).
validate_architecture_rules: 0 erros, 1 aviso (R005 complexidade) mantido.
sync_backlog: 1 história e 4 tarefas (TASK-BIL-01..04) no backlog.
Confirmar: provedor de boleto (ADR-007 em Proposto).
```
