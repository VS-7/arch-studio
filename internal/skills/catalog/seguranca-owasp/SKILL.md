---
name: seguranca-owasp
description: Aplica o OWASP Top 10 a todo código que recebe entrada externa ou expõe dados — validação na fronteira, autorização negada por padrão, SQL parametrizado, saída codificada, CSRF, erros opacos e rate limit. Use ao iniciar tarefas de backend, frontend ou integração.
category: seguranca
version: 1.0.0
trigger: ao_iniciar_tarefa
applies_to:
  tiers: [backend, frontend, integration]
checks:
  - id: validacao-na-fronteira
    text: Toda entrada externa (body, query, path, header, mensagem de fila) é validada por tipo, tamanho e formato antes de chegar ao domínio.
    required: true
  - id: autorizacao-por-endpoint
    text: Todo endpoint novo ou alterado verifica autenticação e autorização explicitamente, e rotas sem regra respondem 401 ou 403.
    required: true
  - id: sql-parametrizado
    text: Nenhuma query é montada por concatenação ou interpolação de dados de entrada; só placeholders do driver ou do ORM.
    required: true
  - id: saida-codificada
    text: Dados de usuário renderizados em HTML passam pelo escape padrão do framework, sem innerHTML, dangerouslySetInnerHTML ou template.HTML com conteúdo não sanitizado.
    required: true
  - id: erros-opacos
    text: Respostas de erro ao cliente não contêm stack trace, SQL, caminho de arquivo nem mensagem crua de dependência.
    required: true
  - id: csrf-em-sessao-cookie
    text: Rotas que mudam estado com sessão em cookie exigem token CSRF ou cookie SameSite com checagem de Origin (na se a autenticação não usa cookie).
    required: false
  - id: rate-limit-autenticacao
    text: Login, cadastro, recuperação de senha e emissão de token têm limite de tentativas por IP e por conta.
    required: false
  - id: logs-sem-dados-sensiveis
    text: Logs novos não registram senha, token, documento pessoal, cartão nem corpo completo de requisição.
    required: false
  - id: sast-sem-achados
    text: A varredura estática com as regras OWASP não aponta achados nos arquivos alterados.
    command: semgrep scan --config p/owasp-top-ten --error
    required: false
---

# Segurança OWASP

## Quando usar

Ao iniciar qualquer tarefa que crie ou altere endpoint, handler, consumidor de fila, integração externa ou tela que exiba dados de usuário. Leia antes de codar: estas regras definem a forma do código, não uma revisão posterior. Valem para qualquer linguagem; os exemplos usam Go e TypeScript.

## Regras

1. **Valide na fronteira (A03/A04).** Decodifique para um tipo estrito e valide tipo, tamanho, faixa e formato no handler ou consumidor; rejeite com 400/422. O domínio só recebe valores já validados. Limite o corpo (`http.MaxBytesReader`, `express.json({ limit: '100kb' })`).
2. **Negue por padrão (A01).** Todo endpoint passa pelo middleware de autenticação; rotas públicas ficam numa lista explícita. Autorize por recurso (o usuário é dono do pedido `:id`?), não só por papel — evite IDOR.
3. **Só SQL parametrizado (A03).** Placeholders (`$1`, `?`) ou query builder. Coluna ou ordenação dinâmica vem de allowlist.
4. **Codifique a saída (XSS).** Use o escape padrão (JSX, `html/template`). `dangerouslySetInnerHTML`, `innerHTML` e `template.HTML` só com conteúdo sanitizado (ex.: DOMPurify) e comentário justificando. Envie `Content-Security-Policy` ao servir HTML.
5. **CSRF em sessão por cookie.** Cookie de sessão com `HttpOnly`, `Secure` e `SameSite=Lax|Strict`; métodos que mudam estado exigem token CSRF ou checagem de `Origin`. APIs com `Authorization: Bearer` não precisam.
6. **Erros opacos (A05).** Ao cliente: status HTTP, mensagem genérica e id de correlação. Stack, SQL e erro do driver só no log do servidor.
7. **Rate limit em autenticação (A07).** Login, cadastro, recuperação de senha e refresh de token com limite por IP e por conta; responda 429 com `Retry-After`. A mensagem de login não revela se o e-mail existe.
8. **Logs úteis e limpos (A09).** Registre quem, o quê, quando e o resultado de ações sensíveis (login, troca de permissão); nunca senha, token, `Authorization`, cookie, CPF ou cartão.
9. **Criptografia padrão (A02).** Senha com bcrypt ou argon2id; comparação de segredos em tempo constante; nada de algoritmo caseiro.
10. **SSRF (A10).** URL vinda do usuário para chamada de saída passa por allowlist de host; bloqueie IPs internos e de metadados de nuvem.

## Como verificar

- `validacao-na-fronteira`: cada handler novo tem teste com entrada inválida que espera 400/422.
- `autorizacao-por-endpoint`: teste sem credencial espera 401; teste com usuário de outro dono espera 403/404.
- `sql-parametrizado`: nenhuma query no diff é montada com `Sprintf`, `+` ou template string.
- `saida-codificada`: nenhum `dangerouslySetInnerHTML`, `innerHTML` ou `template.HTML` novo sem sanitização.
- `erros-opacos`: teste que força erro interno confere que o corpo da resposta não traz detalhes.
- `csrf-em-sessao-cookie`: POST sem token/Origin válido espera 403; `na` se não há cookie de sessão.
- `rate-limit-autenticacao`: teste que excede o limite espera 429.
- `logs-sem-dados-sensiveis`: revise cada chamada de log adicionada no diff.
- `sast-sem-achados`: `semgrep scan --config p/owasp-top-ten --error` termina com código 0 (`na` se o Semgrep não estiver disponível, dizendo isso na evidência).

## Exemplos

```go
// ❌ injeção
db.Query(fmt.Sprintf("SELECT * FROM users WHERE email = '%s'", email))
// ✅ parametrizado
db.QueryContext(ctx, "SELECT id, name FROM users WHERE email = $1", email)
```

```go
// ❌ vaza detalhes ao cliente
http.Error(w, err.Error(), http.StatusInternalServerError)
// ✅ opaco para o cliente, completo no log
slog.ErrorContext(ctx, "buscar pedido", "err", err, "req_id", reqID)
http.Error(w, "erro interno", http.StatusInternalServerError)
```
