---
name: segredos-e-config
description: Garante que segredos só entram por variável de ambiente ou cofre, que .env nunca é versionado, que .env.example acompanha toda variável nova e que nada vaza em logs, sessões ou memórias. Vale em toda tarefa, do primeiro arquivo ao commit.
category: seguranca
version: 1.0.0
trigger: sempre
checks:
  - id: sem-segredo-no-repositorio
    text: A varredura de segredos no repositório não encontra nenhum vazamento.
    command: gitleaks detect --no-banner --redact
    required: true
  - id: env-ignorado
    text: O arquivo .env está coberto pelo .gitignore e não é rastreado pelo Git.
    command: git check-ignore -q .env
    required: false
  - id: env-example-atualizado
    text: Toda variável de ambiente nova lida pelo código aparece em .env.example com valor fictício ou vazio.
    required: true
  - id: arch-sem-segredo
    text: Nenhum segredo foi gravado em .arch/sessions/, .arch/memory/, checkpoints ou mensagens de commit.
    command: gitleaks detect --no-banner --redact --no-git --source .arch
    required: false
  - id: logs-sem-segredo
    text: Nenhum log, erro ou saída de CLI adicionado imprime segredo, token, header Authorization ou o ambiente inteiro.
    required: true
  - id: config-falha-cedo
    text: Variável obrigatória ausente faz a aplicação parar na inicialização com mensagem que cita o nome da variável, nunca o valor.
    required: false
---

# Segredos e configuração

## Quando usar

Sempre. Qualquer tarefa pode introduzir uma credencial: uma string de conexão, uma chave de API num teste, um token colado num checkpoint. Aplique estas regras ao escrever configuração, ao registrar sessão ou memória e antes de cada commit.

## Regras

1. **Segredo só por ambiente ou cofre.** Leia de variável de ambiente ou do secret manager adotado (Vault, AWS Secrets Manager, Doppler, secrets do CI). Nunca literal no código, em teste, em YAML versionado, no `Dockerfile` ou no `docker-compose.yml`.
2. **`.env` nunca é versionado.** Garanta `.env`, `.env.local` e `.env.*.local` no `.gitignore`. Se um `.env` já foi commitado: `git rm --cached .env` e avise o humano de que o segredo precisa ser **rotacionado** — apagar do histórico não basta.
3. **`.env.example` anda junto.** Toda variável nova entra em `.env.example` no mesmo commit, com valor fictício ou vazio e um comentário curto:
   ```dotenv
   # Chave HMAC para assinar JWT (mín. 32 bytes). Gere com: openssl rand -hex 32
   JWT_SECRET=
   DATABASE_URL=postgres://app:app@localhost:5432/app?sslmode=disable
   ```
4. **Falhe cedo, sem ecoar valor.** Valide a configuração na inicialização; a mensagem cita o nome (`JWT_SECRET ausente`), nunca o conteúdo.
5. **Nada de segredo em log.** Não logue `os.Environ()`, `process.env`, headers `Authorization`/`Cookie`, URL com credencial nem a struct de config inteira. Redija: `token=***`.
6. **A memória do projeto é lida pelo time.** `.arch/sessions/` e `.arch/memory/` são versionados. Em `save_checkpoint`, `log_session`, `remember`, `create_backlog_item` e mensagens de commit, cite o segredo pelo nome da variável (`usa STRIPE_API_KEY`), nunca pelo valor.
7. **Teste com valores falsos.** Fixtures usam valores obviamente falsos (`segredo-de-teste`); nunca copie credencial real de outro ambiente.
8. **Varra antes de commitar.** Rode o gitleaks antes de `propose_commit`. Achado bloqueia o commit até ser removido e, se real, rotacionado.

## Como verificar

- `sem-segredo-no-repositorio`: `gitleaks detect --no-banner --redact` termina com código 0.
- `env-ignorado`: `git check-ignore -q .env` termina com código 0.
- `env-example-atualizado`: toda leitura nova no diff (`os.Getenv`, `process.env.`, `os.environ`) tem a variável em `.env.example`.
- `arch-sem-segredo`: `gitleaks detect --no-banner --redact --no-git --source .arch` termina com código 0; revise também as mensagens de commit da branch.
- `logs-sem-segredo`: revise cada chamada de log ou print adicionada no diff.
- `config-falha-cedo`: teste que inicia sem a variável espera erro citando o nome dela.

## Exemplos

```go
// ❌ literal no código
const apiKey = "sk_live_xxx"
// ✅ ambiente + falha cedo
key := os.Getenv("STRIPE_API_KEY")
if key == "" {
	return errors.New("config: STRIPE_API_KEY ausente")
}
```

- ❌ checkpoint: `next_step: "testar com o token eyJhbGci... do staging"`
- ✅ checkpoint: `next_step: "testar com token assinado pelo JWT_SECRET local"`
