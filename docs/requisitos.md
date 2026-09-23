# Requisitos — archcode-studio

> Documento gerenciado pelo ArchCode Studio. Editável por humanos e por agentes de IA via MCP.

## Visão Geral

Descreva aqui o objetivo do sistema, o problema de negócio que ele resolve e o público-alvo.
Este texto é lido pelos agentes de IA como contexto de produto antes de qualquer implementação.

## Requisitos Funcionais

### RF001 — Autenticação de usuários

- **Prioridade:** Alta
- **Status:** pending
- **Componentes:** node-core-api, node-postgres

O sistema deve autenticar usuários por e-mail e senha, emitindo um token JWT com validade configurável.

### RF002 — Consulta do perfil autenticado

- **Prioridade:** Média
- **Status:** pending
- **Componentes:** node-core-api

Usuários autenticados devem conseguir consultar seus próprios dados de perfil e papéis de acesso.

## Requisitos Não Funcionais

### RNF001 — Toda rota privada exige token JWT válido

- **Prioridade:** Alta
- **Status:** pending
- **Componentes:** node-api-gateway, node-core-api

Requisições sem token ou com token expirado devem retornar HTTP 401 sem vazar detalhes internos.

### RNF002 — Tempo de resposta P95 abaixo de 300ms

- **Prioridade:** Média
- **Status:** pending
- **Componentes:** node-core-api, node-redis

Endpoints de leitura devem responder em menos de 300ms no percentil 95 sob carga nominal.

