---
name: definicao-de-pronto
description: Definição de Pronto antes de abrir PR — critérios cobertos por testes verdes, lint limpo, checks das skills reportados no complete_task, docs e ADR atualizados, sessão registrada, commit na convenção e arquitetura válida. Use antes de todo PR.
category: padronizacao
version: 1.0.0
trigger: antes_do_pr
checks:
  - id: criterios-cobertos
    text: Todos os critérios de aceite da tarefa estão cobertos por testes automatizados que passam.
    required: true
  - id: lint-e-formatacao
    text: Formatação e lint da stack do projeto terminam sem apontamentos.
    required: true
  - id: checks-das-skills-reportados
    text: Todos os checks obrigatórios das skills ativas da tarefa foram enviados ao complete_task com resultado e evidência.
    required: false
  - id: docs-e-adr-atualizados
    text: Requisitos, casos de uso, README ou ADR foram atualizados quando uma decisão ou comportamento documentado mudou.
    required: true
  - id: sessao-registrada
    text: A sessão foi registrada com log_session, com arquivos, comandos e próximos passos.
    required: true
  - id: commit-na-convencao
    text: As mensagens de commit da branch seguem a convenção do projeto.
    command: archcode-studio git lint
    required: false
  - id: arquitetura-valida
    text: O linter de arquitetura termina sem erros.
    command: archcode-studio validate
    required: true
---

# Definição de Pronto

## Quando usar

Antes de abrir o PR — ou de mover a tarefa para revisão — de qualquer tarefa. Ela só está pronta quando todos os itens abaixo são verdadeiros **e comprovados**; "funciona na minha máquina" não é evidência.

## Regras

1. **Critérios cobertos.** Cada critério de aceite do arquivo da tarefa tem teste que passa (skill `testes-e-aceite`). Rode a suíte completa dos componentes afetados, não só os testes novos.
2. **Lint e formatação limpos** com as ferramentas da stack — Go: `gofmt -l .` vazio e `go vet ./...`; TypeScript: `npx tsc --noEmit` e `npm run lint`. Não desative regras para passar.
3. **Checks de todas as skills ativas.** Chame `list_skills(task_id)` e reporte cada check no `complete_task`: `ok` com evidência, `na` com motivo, `fail` com o que falta. O `complete_task` recusa se um check obrigatório faltar ou falhar — corrija, não contorne.
   ```json
   {"task_id": "TASK-API-01", "checks": [
     {"skill": "definicao-de-pronto", "check": "arquitetura-valida", "result": "ok",
      "evidence": "archcode-studio validate → 0 erros, 1 aviso (R005)"},
     {"skill": "seguranca-owasp", "check": "csrf-em-sessao-cookie", "result": "na",
      "evidence": "API usa Bearer; não há cookie de sessão"}
   ], "notes": "..."}
   ```
4. **Docs e decisões em dia.** Comportamento de requisito mudou → `upsert_requirement`/`upsert_use_case`. Decisão arquitetural (biblioteca, padrão, trade-off) → `upsert_adr`. Contrato mudou → modelo via ferramentas. Forma de rodar ou configurar mudou → README e `.env.example`.
5. **Sessão registrada.** `log_session` com `summary`, `done`, `decisions`, `next_steps`, `blockers`, `files` e `commands` executados.
6. **Commit na convenção.** Gere a mensagem com `propose_commit` (ex.: `Sprint 01 - Implementa emissão de JWT [TASK-API-01]`) e valide com `archcode-studio git lint`.
7. **Arquitetura válida.** `archcode-studio validate` sem erros; avisos novos explicados no PR.
8. **Nada pendente escondido.** TODO novo vira `create_backlog_item`; sem `console.log`, print de depuração ou arquivo temporário no diff.

## Como verificar

- `criterios-cobertos`: a evidência lista critério → teste → resultado.
- `lint-e-formatacao`: os comandos da regra 2 terminam com código 0 (gofmt com saída vazia).
- `checks-das-skills-reportados`: o payload do `complete_task` tem todos os checks obrigatórios de cada skill de `list_skills(task_id)` e foi aceito sem recusa.
- `docs-e-adr-atualizados`: o diff inclui a atualização correspondente, ou `na` com "sem mudança de decisão ou comportamento documentado".
- `sessao-registrada`: `log_session` foi chamado nesta sessão, antes do PR.
- `commit-na-convencao`: `archcode-studio git lint` termina com código 0.
- `arquitetura-valida`: `archcode-studio validate` termina com código 0.
