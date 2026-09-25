---
name: archcode-sprint
description: Planeja e conduz sprints no ArchCode — backlog priorizado, plano que cabe na capacidade e é confirmado por humano, progresso lido de dados reais e fechamento com relatório, carry-over, CHANGELOG e tag sugerida. Vale sempre que o assunto for planejar, acompanhar ou fechar sprint.
category: studio
version: 1.0.0
scope: sprint
trigger: sempre
checks:
  - id: nome-e-objetivo
    text: O sprint se chama Sprint NN, com dois dígitos, e tem objetivo escrito em uma frase.
    required: true
  - id: cabe-na-capacidade
    text: A soma das estimativas dos itens do sprint não ultrapassa a capacidade (pessoas × horas por dia × dias úteis).
    required: true
  - id: plano-confirmado-por-humano
    text: O plano proposto por plan_sprint foi confirmado por um humano antes de o sprint começar.
    required: true
  - id: progresso-consultado
    text: O progresso foi lido com get_sprint ou pelo CLI antes de qualquer relato.
    command: archcode-studio sprint status
    required: false
  - id: fechamento-completo
    text: O fechamento gerou docs/sprints/sprint-NN.md, carry-over dos itens não concluídos e seção no CHANGELOG.md.
    required: true
  - id: tag-e-release-humanos
    text: O agente não criou tag nem release; a tag sprint-NN foi apenas sugerida.
    command: git tag --list 'sprint-*'
    required: true
---

# Sprints no ArchCode

## Quando usar

Sempre que o pedido envolver planejar, iniciar, acompanhar ou fechar um sprint — ou relatar progresso a alguém. O agente propõe e executa a mecânica; escopo, início e publicação são decisões humanas.

## Regras

1. **Nome e objetivo.** Sprints se chamam `Sprint 01`, `Sprint 02`… O objetivo é uma frase de resultado verificável — "Usuário consegue se cadastrar e entrar com JWT." —, não uma lista de tarefas.
2. **Parta do backlog.** `list_backlog` para ver os itens priorizados. Item sem estimativa ou sem critério de aceite não entra: refine antes (ou modele com a skill `archcode-modelagem`).
3. **Capacidade primeiro.** capacidade = pessoas × horas/dia × dias úteis, descontando feriados e ausências. Ex.: 3 pessoas × 6 h × 9 dias = 162 h. Deixe ~15–20% de folga para bugs e revisão.
4. **`plan_sprint` propõe, humano decide.** A ferramenta sugere os itens que cabem na capacidade, respeitando prioridade e dependências (banco → domínio → serviços → integrações → interface). Apresente itens, horas, folga e riscos, e **aguarde confirmação** antes de iniciar.
5. **CLI.** `archcode-studio sprint new` cria, `sprint start` inicia, `sprint status` mostra o progresso e `sprint close` fecha.
6. **Acompanhe com dados.** `get_sprint` (ou `archcode-studio sprint status`) antes de relatar: concluído × planejado, tarefas em andamento sem checkpoint recente, bloqueios. Nunca relate de memória.
7. **Escopo congelado.** Item novo no meio do sprint vai para o backlog (`create_backlog_item`); só entra se um humano trocá-lo por algo de tamanho equivalente.
8. **Fechamento** (`archcode-studio sprint close`) produz:
   - relatório em `docs/sprints/sprint-NN.md` — objetivo atingido?, entregas, métricas, decisões, bloqueios;
   - **carry-over** dos itens não concluídos para o próximo sprint ou backlog, preservando checkpoints;
   - seção do sprint no `CHANGELOG.md`;
   - tag sugerida `sprint-NN`.
9. **Tag e release são humanos.** Informe o comando sugerido — `git tag -a sprint-01 -m "Sprint 01"` — e pare. Nunca crie tag, release nem faça merge.
10. **Retrospectiva vira memória.** Lições do sprint → `remember` com `type: armadilha` ou `convencao`; mudança de processo relevante → `upsert_adr`.

## Como verificar

- `nome-e-objetivo`: `get_sprint` mostra nome `Sprint NN` e objetivo de uma frase.
- `cabe-na-capacidade`: soma das estimativas ≤ capacidade calculada pela regra 3, com o cálculo na evidência.
- `plano-confirmado-por-humano`: há confirmação explícita do humano na conversa ou no `log_session` antes de `sprint start`.
- `progresso-consultado`: `archcode-studio sprint status` (ou `get_sprint`) foi executado antes do relato.
- `fechamento-completo`: `docs/sprints/sprint-NN.md` existe, o `CHANGELOG.md` tem a seção do sprint e os itens abertos aparecem no próximo sprint ou no backlog.
- `tag-e-release-humanos`: `git tag --list 'sprint-*'` não mostra tag criada pelo agente nesta sessão.

## Exemplos

- ❌ Objetivo: "Fazer TASK-API-01, TASK-API-02 e TASK-WEB-01."
- ✅ Objetivo: "Cliente consegue pagar um pedido com cartão em sandbox."
- ❌ "O sprint está quase pronto" (sem consulta). ✅ "Sprint 03: 11/14 tarefas concluídas (79%), 2 em revisão, 1 bloqueada (TASK-BIL-02)."
