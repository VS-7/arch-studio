---
name: fronteiras-arquiteturais
description: Garante que cada componente só chama componentes conectados a ele no diagrama, pelo protocolo e endpoints declarados, sem acessar banco alheio, e que mudança de contrato passa antes pelas ferramentas MCP. Vale sempre que o código cruzar a fronteira de um componente.
category: organizacao
version: 1.0.0
trigger: sempre
checks:
  - id: chamada-tem-conexao
    text: Todo cliente de rede ou de fila adicionado aponta para um componente conectado ao componente atual no diagrama.
    required: true
  - id: endpoint-no-contrato
    text: Todo endpoint exposto ou consumido existe em api/endpoints.yaml com o mesmo método, caminho e auth.
    required: true
  - id: banco-exclusivo
    text: Nenhum componente lê ou escreve em banco, cache ou storage que pertence a outro componente.
    required: true
  - id: contrato-atualizado-por-ferramenta
    text: Toda mudança de contrato foi aplicada ao modelo com connect_nodes ou update_node_metadata antes ou junto do código.
    required: true
  - id: diagrama-sem-edicao-manual
    text: Nenhum arquivo em .arch/diagrams/ foi editado à mão.
    required: true
---

# Fronteiras arquiteturais

## Quando usar

Sempre que o código cruzar a fronteira de um componente: chamar outra API, publicar ou consumir mensagem, ler dados que não são do componente, expor ou alterar endpoint. As conexões do diagrama (`get_system_context`) e os contratos em `api/endpoints.yaml` são a lista completa do que é permitido.

## Regras

1. **Só fale com vizinhos.** O componente A só chama B se existir aresta A → B no diagrama. Chamada sem aresta é violação, mesmo que "funcione".
2. **Pelo protocolo e endpoint declarados.** Se a aresta é `REST` com `POST /api/v1/orders`, o cliente usa exatamente esse método e caminho. Não consuma outro endpoint do mesmo serviço sem que ele esteja em `api/endpoints.yaml`. Aresta `AMQP` não vira chamada HTTP síncrona.
3. **Banco é privado.** Cada banco, cache ou storage pertence ao componente conectado a ele. Precisa de dado de outro componente? Use a API ou o evento dele. Nunca compartilhe tabela, schema ou credencial entre serviços.
4. **Contrato primeiro.** Mudar request, response, status ou auth de um endpoint é mudar o contrato: atualize a conexão com `connect_nodes` (campo `endpoints`) e os metadados do componente com `update_node_metadata` **antes** de mexer no código. Depois implemente o servidor e, por último, o cliente.
5. **Nunca edite `.arch/diagrams/macro.json` à mão.** Ele é a fonte da verdade visual, com posições e invariantes que só as ferramentas preservam. Use `add_architecture_node`, `connect_nodes`, `update_node_metadata` e `remove_architecture_node`.
6. **Compatibilidade.** Mudança que quebra consumidor (remover campo, mudar tipo) exige nova versão de rota (`/api/v2/...`) ou período de convivência; registre a decisão com `upsert_adr`.
7. **Faltou aresta? Pare.** Não improvise: modele pelo prompt `model_new_feature` (skill `archcode-modelagem`), rode `sync_backlog` e retome.

## Como verificar

- `chamada-tem-conexao`: para cada cliente de rede ou fila no diff, a evidência cita a aresta de `get_system_context`.
- `endpoint-no-contrato`: cada rota registrada ou chamada no diff tem entrada com o mesmo `method`, `path` e `auth` em `api/endpoints.yaml`.
- `banco-exclusivo`: nenhuma string de conexão, migração ou repositório no diff aponta para o banco de outro componente.
- `contrato-atualizado-por-ferramenta`: a mudança de contrato aparece no modelo e em `api/endpoints.yaml` no mesmo PR, feita por chamada de ferramenta.
- `diagrama-sem-edicao-manual`: toda mudança em `.arch/diagrams/` no diff corresponde a uma chamada de ferramenta listada em `commands` do `log_session`.

## Exemplos

- ❌ `billing` executa `SELECT * FROM users` no banco do `core-api`.
- ✅ `billing` chama `GET /api/v1/users/{id}` do `core-api` (aresta `billing → core-api`, `REST`, `JWT`).
- ❌ Acrescentar `{"source": "web", "target": "db"}` em `macro.json`.
- ✅ `connect_nodes(source_id: "web-app", target_id: "api-gateway", protocol: "REST", port: 443, security: "JWT")`.
