---
name: testes-e-aceite
description: Transforma cada critério de aceite da tarefa em pelo menos um teste automatizado com o nome do critério, exige suíte verde e registra comando e resultado como evidência no complete_task. Use ao iniciar qualquer tarefa, antes de implementar.
category: organizacao
version: 1.0.0
trigger: ao_iniciar_tarefa
checks:
  - id: criterio-tem-teste
    text: Cada critério de aceite da tarefa tem pelo menos um teste automatizado cujo nome identifica o critério.
    required: true
  - id: gwt-estruturado
    text: Critérios Given-When-Then viraram testes com as etapas Dado, Quando e Então explícitas.
    required: false
  - id: suite-verde
    text: A suíte de testes dos componentes afetados passa por completo, sem testes pulados novos.
    required: true
  - id: evidencia-registrada
    text: O comando exato e o resumo do resultado dos testes estão na evidência enviada ao complete_task.
    required: true
  - id: regressao-para-bug
    text: Toda correção de bug inclui teste que falha sem a correção e passa com ela.
    required: false
---

# Testes e critérios de aceite

## Quando usar

Ao iniciar qualquer tarefa, logo após `claim_task` e antes de escrever código de produção. Os critérios de aceite estão no arquivo da tarefa (`.arch/plan/tasks/<ID>.md`) e chegam também por `resume_work` e `list_backlog`; eles definem o que "funcionar" significa.

## Regras

1. **Liste os critérios primeiro.** Registre-os no primeiro `save_checkpoint`. Critério ambíguo: pergunte ao humano ou registre a interpretação com `remember` (`type: decisao`) antes de codar.
2. **Um critério, pelo menos um teste**, com nome que identifica o critério:
   - Go: `func TestTaskAPI01_LoginComSenhaInvalidaRetorna401(t *testing.T)`
   - TS: `it('TASK-API-01: login com senha inválida retorna 401', ...)`
3. **Given-When-Then vira estrutura visível:**
   ```go
   t.Run("dado usuário ativo, quando envia senha errada, então recebe 401", func(t *testing.T) {
   	// Dado
   	srv := novoServidor(t, comUsuario("ana@ex.com", "certa"))
   	// Quando
   	res := srv.Post("/api/v1/auth/login", `{"email":"ana@ex.com","password":"errada"}`)
   	// Então
   	if res.Code != http.StatusUnauthorized {
   		t.Fatalf("status = %d, quer 401", res.Code)
   	}
   })
   ```
4. **Teste comportamento, não implementação.** Teste pela fronteira (handler, caso de uso) com dependências externas substituídas por fakes; use integração com banco real quando o critério envolve persistência.
5. **Rode a suíte dos componentes afetados**, não só o teste novo: `go test ./...`, `npm test`, `pytest`. Nenhum `t.Skip`, `it.skip` ou `xit` novo sem justificativa.
6. **Bug começa pelo teste de regressão.** Escreva o teste que reproduz o bug, veja-o falhar, corrija, veja-o passar.
7. **Vermelho não conclui.** Com teste falhando, não chame `complete_task`: registre `failing_tests` no `save_checkpoint` e continue — ou pare e declare o bloqueio em `blockers` do `log_session`.
8. **Evidência exata**, copiada da saída real, no `complete_task`:
   `{"skill": "testes-e-aceite", "check": "suite-verde", "result": "ok", "evidence": "go test ./internal/auth/... → ok (0.8s), 0 falhas"}`

## Como verificar

- `criterio-tem-teste`: a evidência lista, para cada critério do arquivo da tarefa, o nome do teste correspondente.
- `gwt-estruturado`: os testes de critérios GWT têm os blocos Dado/Quando/Então (comentários ou subtestes).
- `suite-verde`: o comando de testes do projeto (`go test ./...`, `npm test`, `pytest`) termina com código 0.
- `evidencia-registrada`: a evidência do `complete_task` contém o comando e o resultado copiados da saída.
- `regressao-para-bug`: em tarefa de bug, o teste de regressão falha sem a correção e passa com ela (`na` se a tarefa não é bug).

## Exemplos

- ❌ Critério "senha expira em 90 dias" coberto só por `TestUserService` genérico.
- ✅ `TestTaskAUTH03_SenhaExpiradaApos90DiasExigeTroca`.
- ❌ Evidência: "testes ok". ✅ Evidência: "`npm test -- src/auth` → 12 passed, 0 failed".
