# ADR-001 — Autenticação baseada em JWT stateless

- **Status:** Aceito
- **Data:** 2026-09-23

## Contexto

O sistema precisa autenticar clientes web e mobile sem manter sessão em memória no servidor, permitindo escalar horizontalmente a Core API sem sticky sessions.

## Decisão

Adotar JWT assinado (HS256 na fase inicial, migrando para RS256 quando houver múltiplos emissores), com validade curta de 1 hora e refresh token rotativo persistido no PostgreSQL.

## Consequências

Positivo: escala horizontal trivial e validação local barata. Negativo: revogação imediata exige lista de bloqueio em Redis, já prevista na arquitetura.
