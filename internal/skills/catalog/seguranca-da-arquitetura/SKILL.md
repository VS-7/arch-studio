---
name: seguranca-da-arquitetura
description: Confere, antes do PR, que o código implementa exatamente as conexões do diagrama — protocolo, porta e segurança declarados (JWT, mTLS, TLS…), nenhum cliente falando com banco e nenhuma conexão improvisada. Use antes do PR de tarefa que toque comunicação entre componentes.
category: seguranca
version: 1.0.0
trigger: antes_do_pr
checks:
  - id: conexoes-do-diagrama
    text: Cada chamada entre componentes criada ou alterada corresponde a uma conexão existente em get_system_context.
    required: true
  - id: seguranca-declarada-implementada
    text: O mecanismo de segurança declarado na conexão está implementado no lado que recebe e coberto por teste que rejeita a chamada sem credencial válida.
    required: true
  - id: protocolo-e-porta
    text: Protocolo e porta usados no código e na configuração batem com os metadados da conexão.
    required: false
  - id: cliente-sem-banco
    text: Nenhum componente do tipo client acessa banco, cache ou storage diretamente.
    required: true
  - id: arquitetura-valida
    text: O linter de arquitetura termina sem erros.
    command: archcode-studio validate
    required: true
  - id: sem-conexao-improvisada
    text: Nenhuma dependência entre componentes existe no código sem ter sido modelada antes no diagrama.
    required: true
---

# Segurança da arquitetura

## Quando usar

Antes de abrir o PR de qualquer tarefa que crie ou altere comunicação entre componentes: cliente HTTP/gRPC, produtor ou consumidor de fila, acesso a banco/cache/storage, webhook, chamada a serviço externo. O diagrama do ArchCode é o contrato de segurança aprovado; o código deve implementá-lo — nem mais, nem menos.

## Regras

1. **Leia as conexões reais.** Chame `get_system_context` e anote, para cada conexão tocada: origem, destino, `protocol`, `port`, `security` e `endpoints`. Não confie em memória de sessão anterior.
2. **Implemente o que o diagrama declara.**

   | `security` | O código deve ter |
   | --- | --- |
   | `JWT` | assinatura validada, `exp`, `iss`/`aud` conferidos, algoritmo fixo (nunca aceitar `alg: none`) |
   | `mTLS` | certificado de cliente exigido e verificado (`tls.RequireAndVerifyClientCert` + `ClientCAs`) |
   | `TLS` | canal cifrado (`https://`, `sslmode=verify-full`, `rediss://`), sem `InsecureSkipVerify: true` |
   | `API Key` | chave lida do ambiente, enviada em header, comparada em tempo constante |
   | vazio | pare e pergunte — sem segurança só se o diagrama justificar (rede interna isolada) |

3. **Protocolo e porta coerentes.** Se a aresta diz `gRPC` na `50051`, não exponha REST na 8080. A porta vem de configuração com default igual ao diagrama.
4. **Cliente nunca fala com banco.** Componentes `client` (web, mobile, desktop) só chamam gateway ou serviços. Nenhum driver de banco, string de conexão ou chave de storage no bundle do cliente (regra `R006-client-direct-database`).
5. **Sem conexão improvisada.** Se o código precisa de uma conexão que não existe no diagrama, **pare**. Modele primeiro pelo prompt `model_new_feature` (skill `archcode-modelagem`), rode `sync_backlog` e só então implemente. Não existe atalho "temporário".
6. **Valide.** `archcode-studio validate` com erros como `R010-missing-authentication`, `R008-edge-without-protocol` ou `R006-client-direct-database` bloqueia o PR.
7. **Declare o impacto.** No corpo do PR (`prepare_pull_request`), liste as conexões implementadas e o mecanismo de segurança de cada uma.

## Como verificar

- `conexoes-do-diagrama`: para cada cliente de rede ou fila no diff, a evidência cita a aresta correspondente (origem → destino, protocolo).
- `seguranca-declarada-implementada`: teste que chama sem credencial (ou com token/certificado inválido) e espera rejeição.
- `protocolo-e-porta`: configuração, `Dockerfile` e `docker-compose.yml` batem com `protocol` e `port` da aresta.
- `cliente-sem-banco`: nenhum driver de banco nas dependências de componentes `client`, e o validate não aponta `R006`.
- `arquitetura-valida`: `archcode-studio validate` termina com código 0.
- `sem-conexao-improvisada`: toda aresta citada em `conexoes-do-diagrama` existe em `get_system_context`; as criadas nesta tarefa aparecem no PR como impacto na arquitetura.

## Exemplos

- Diagrama: `Gateway → Pedidos (gRPC, 50051, mTLS)`.
  - ❌ `grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))`
  - ✅ `grpc.NewClient(addr, grpc.WithTransportCredentials(credentials.NewTLS(cfgComCertCliente)))`
- ❌ App React com `DATABASE_URL` no build gravando direto na tabela.
- ✅ App React chama `POST /api/v1/orders` do gateway, conforme `api/endpoints.yaml`.
