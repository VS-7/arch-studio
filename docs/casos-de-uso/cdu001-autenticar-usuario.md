# CDU001 — Autenticar Usuário

- **Atores:** Usuário final
- **Componentes:** node-web-app, node-core-api, node-postgres
- **Complexidade:** medium
- **Horas Estimadas:** 16
- **Prioridade:** Alta
- **Status:** pending

## Pré-condições

- Usuário previamente cadastrado
- Conta ativa e não bloqueada

## Fluxo Principal

1. O usuário informa e-mail e senha na tela de login
2. A interface envia POST /api/v1/auth/login para a API
3. A API valida as credenciais contra o hash armazenado no PostgreSQL
4. A API emite um JWT assinado com validade de 1 hora
5. A interface armazena o token e redireciona para o painel

## Fluxos Alternativos

- Usuário opta por login social: o fluxo delega ao provedor OAuth2 e retorna ao passo 4

## Exceções

- Credenciais inválidas: a API retorna 401 e a interface exibe mensagem genérica
- Cinco tentativas falhas em 10 minutos: a conta entra em bloqueio temporário

## Regras de Negócio

- Senhas são armazenadas apenas como hash com algoritmo de custo adaptativo
- Mensagens de erro nunca revelam se o e-mail existe na base

## Critérios de Aceite

- **Given** um usuário cadastrado com credenciais válidas **When** envia `POST /api/v1/auth/login` **Then** recebe HTTP 200 com `{"token": "<jwt>", "expires_in": 3600}`.
- **Given** um usuário com senha incorreta **When** envia `POST /api/v1/auth/login` **Then** recebe HTTP 401 sem indicar qual campo falhou.

