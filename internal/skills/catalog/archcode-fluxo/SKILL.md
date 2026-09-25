---
name: archcode-fluxo
description: Protocolo de trabalho num projeto ArchCode via MCP — resume_work primeiro, claim_task, skills carregadas, checkpoints a cada passo, escopo restrito à tarefa, complete_task com evidências, commit na convenção e log_session antes de parar. Vale sempre, em toda sessão.
category: studio
version: 1.0.0
trigger: sempre
checks:
  - id: resume-work-primeiro
    text: A sessão chamou resume_work antes de qualquer leitura de código ou edição.
    required: false
  - id: tarefa-reservada
    text: A tarefa foi reservada com claim_task e o trabalho está na branch sprint-NN/<ID>-<slug> criada por ela.
    command: git branch --show-current
    required: true
  - id: skills-carregadas
    text: Todas as skills retornadas por list_skills para a tarefa foram lidas com get_skill antes de codar.
    required: false
  - id: checkpoints-salvos
    text: save_checkpoint foi chamado após cada passo significativo, com next_step e failing_tests atualizados.
    required: false
  - id: escopo-da-tarefa
    text: O diff contém apenas o escopo da tarefa reservada, e o que foi descoberto fora dele virou create_backlog_item.
    required: true
  - id: plano-e-diagrama-por-ferramenta
    text: Nenhum arquivo em .arch/diagrams/ ou .arch/plan/ foi editado à mão quando existe ferramenta para a mudança.
    required: false
  - id: autonomia-respeitada
    text: Commit, push e PR seguiram o nível de autonomia do projeto, e merge, tag e release ficaram para humanos.
    required: true
  - id: sessao-registrada
    text: log_session foi chamado antes de encerrar a sessão.
    required: false
---

# Fluxo de trabalho no ArchCode

## Quando usar

Sempre, em toda sessão de agente num projeto gerenciado pelo ArchCode Studio. O estado do trabalho — sprint, tarefas, donos, checkpoints, memória — vive em `.arch/` e é compartilhado com pessoas e outros agentes; as ferramentas MCP são a forma segura de lê-lo e alterá-lo.

## Regras

1. **Comece por `resume_work`** (`detail: "brief"`; `"full"` ao retomar trabalho de outra pessoa). Ele traz sprint ativa, suas tarefas em andamento com checkpoint, últimas sessões, estado real do Git (branch, arquivos alterados, ahead/behind), divergências, próxima tarefa pronta, skills e memórias relevantes. **Divergência entre memória e Git: o Git vence** — ajuste o plano ao que está no disco.
2. **Retome antes de começar.** Tarefa sua em andamento: continue do `next_step` do checkpoint. Senão, pegue a próxima tarefa pronta.
3. **Reserve com `claim_task`** (`task_id`, `switch_branch: true`). Ela cria a branch `sprint-NN/<ID>-<slug>` e marca você como dono. Tarefa de outra pessoa: escolha outra. Publicar a reserva (`push`) depende do nível de autonomia.
4. **Carregue as skills.** `list_skills(task_id)` e `get_skill(name)` para cada uma, antes de codar. Use `recall(query)` para convenções e armadilhas do componente.
5. **Escopo é a tarefa reservada.** Bug, dívida, risco ou melhoria fora do escopo vira `create_backlog_item` (`type: bug|debt|spike|security|task`, `title`, descrição com arquivo e linha) — não é feito agora.
6. **Faltou arquitetura? Pare.** Tarefa que exige componente ou conexão fora do diagrama: siga o prompt `model_new_feature` (skill `archcode-modelagem`), rode `sync_backlog` e retome.
7. **`save_checkpoint` a cada passo significativo** — teste escrito, handler pronto, bloqueio encontrado —, não só no fim: `task_id`, `last_step`, `next_step`, `files`, `failing_tests`. Outra sessão ou pessoa deve conseguir continuar só com ele.
8. **Conhecimento durável.** `remember` (`title`, `body`, `type: decisao|convencao|armadilha|contexto|glossario`, `tags`, `components`) para o que o time precisará depois; decisão arquitetural grande vira ADR com `upsert_adr`.
9. **Nunca edite à mão** `.arch/diagrams/*.json` nem `.arch/plan/*` quando existe ferramenta. Nada de segredo em checkpoint, sessão ou memória.
10. **Feche nesta ordem:**
    1. testes, lint e `archcode-studio validate` verdes;
    2. `log_session` (`summary`, `done`, `decisions`, `next_steps`, `blockers`, `files`, `commands`);
    3. `propose_commit(task_id)` → mensagem na convenção, ex.: `Sprint 01 - Implementa emissão de JWT [TASK-API-01]`;
    4. `complete_task(task_id, checks, notes)` com cada check das skills (`ok|fail|na` + evidência); se recusar, corrija o que falta;
    5. `prepare_pull_request(task_ids)` → título e corpo com critérios, checks e impacto na arquitetura.
11. **Respeite o nível de autonomia do projeto.**

    | Nível | Commit | Push | PR |
    | --- | --- | --- | --- |
    | assistido | humano confirma cada commit | humano | humano |
    | supervisionado | agente, local | humano | humano |
    | autônomo | agente | agente | agente abre em draft |

    **Merge, tag e release são sempre humanos.**
12. **Vai parar no meio?** `save_checkpoint` + `log_session` com `blockers` antes de encerrar — sempre.
13. **Legado:** `get_implementation_tasks` e `mark_task_status` seguem válidos em projetos sem backlog e sprint.

## Como verificar

- `resume-work-primeiro`: `resume_work` é a primeira chamada de ferramenta da sessão.
- `tarefa-reservada`: `git branch --show-current` imprime a branch `sprint-NN/<ID>-<slug>` da tarefa.
- `skills-carregadas`: houve `get_skill` para cada skill de `list_skills(task_id)`.
- `checkpoints-salvos`: existe checkpoint para cada passo listado em `done` do `log_session`.
- `escopo-da-tarefa`: cada arquivo do diff pertence ao escopo; o que ficou de fora tem item de backlog.
- `plano-e-diagrama-por-ferramenta`: mudanças em `.arch/diagrams/` e `.arch/plan/` correspondem a chamadas de ferramenta.
- `autonomia-respeitada`: nenhum commit, push ou PR além do permitido pela tabela; nenhum merge, tag ou release.
- `sessao-registrada`: `log_session` foi chamado antes do fim da sessão.
