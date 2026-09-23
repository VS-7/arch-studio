# ArchCode Studio

**Arquitetura de software como código — local-first, versionável em Git e nativa para agentes de IA.**

Projete, documente, valide, precifique e implemente arquiteturas de software com sincronização
bidirecional em tempo real entre **humanos** (canvas drag-and-drop) e **agentes de IA**
(Model Context Protocol). Um único binário, sem banco de dados, sem nuvem, sem lock-in.

```
┌─ Humano ──────────────┐   ┌─ Go Core Engine ────────┐   ┌─ .arch/ + docs/ + api/ ──┐
│ Canvas React Flow     │◄─►│ HTTP + WebSocket        │◄─►│ macro.json               │
│ Editor de requisitos  │   │ File Watcher (fsnotify) │   │ requisitos.md            │
│ Modo Pitch            │   │ Compilador de AI-PRD    │   │ casos-de-uso/            │
│ Painel de preços      │   │ Motor de precificação   │   │ endpoints.yaml           │
└───────────────────────┘   │ Linter de arquitetura   │   │ ai-prd.md                │
                            │ Servidor MCP            │   │ tasks.json               │
┌─ Agente de IA ────────┐   └─────────────────────────┘   └──────────────────────────┘
│ Claude Code / Cursor  │◄─────────► stdio · SSE                  ▲
│ Antigravity / CrewAI  │                                    Git versiona tudo
└───────────────────────┘
```

---

## Por que existe

Diagramas de arquitetura envelhecem mal. Ficam presos em ferramentas proprietárias, divergem do
código e são inúteis para LLMs. O ArchCode Studio resolve isso tratando a arquitetura como **código
versionado**: cada nó, cada rota e cada requisito vive em arquivos de texto na árvore do projeto,
editáveis por humanos no navegador, por desenvolvedores no VS Code e por agentes de IA via MCP —
**simultaneamente, sem conflito e sem corromper o layout visual**.

| Para quem | O que resolve |
| :--- | :--- |
| **Arquiteto / Tech Lead** | Diagrama, contratos de API e Mermaid versionados no mesmo commit do código; linter que aponta falhas estruturais. |
| **Desenvolvedor** | O agente de IA lê a arquitetura real via MCP e implementa na ordem topológica correta, sem alucinar contexto. |
| **Consultor / Software House** | Estimativa de esforço, custo de nuvem e proposta comercial gerados a partir do próprio desenho. |
| **Agente de IA** | Ferramentas atômicas que preservam coordenadas, mantêm rastreabilidade e registram progresso. |

---

## Instalação

```bash
# A partir do código-fonte (requer Go 1.24+ e Node 20+)
git clone https://github.com/archcode/studio archcode-studio
cd archcode-studio
make deps && make build
sudo mv archcode-studio /usr/local/bin/

# Docker
docker run -p 8765:8765 -v "$PWD:/workspace" ghcr.io/archcode/studio
```

O binário final embute todo o frontend: a máquina do usuário **não precisa de Node.js**.

## Deploy no Coolify

O repositório segue o mesmo padrão de deploy do Harmoni (`Dockerfile` + `docker-compose.yml`):

1. Conecte o repositório Git no Coolify (branch `main`).
2. Selecione o tipo de build **Docker Compose**.
3. O volume persistente é `/workspace` (onde ficam `.arch/`, `docs/` e `api/`).
4. Configure as variáveis de ambiente com base no `.env.example`. Defina `ARCHCODE_BASE_URL` com o domínio público para o MCP via SSE funcionar.
5. Inicie o deploy!

Na primeira execução, se `/workspace/.arch/manifest.yaml` não existir, o container roda `init` automaticamente. O healthcheck usa `GET /api/health`.

Localmente: `cp .env.example .env && docker compose up -d --build` → http://localhost:8765

> ⚠ O servidor não tem autenticação própria. Em instância pública, proteja o domínio (ex.: Basic Auth via labels do Traefik no Coolify) ou restrinja o acesso por rede.

## Primeiros passos

```bash
mkdir meu-projeto && cd meu-projeto

archcode-studio init --name "E-Commerce Enterprise" --author "Seu Nome"
archcode-studio serve                 # abre http://127.0.0.1:8765
archcode-studio mcp-config            # imprime como conectar seu agente de IA
```

`init` já cria uma arquitetura de exemplo funcional (Web App → Gateway → API → PostgreSQL/Redis),
com requisitos, um caso de uso e um ADR — para você explorar tudo de imediato.
Use `--empty` para começar do zero.

---

## Estrutura de arquivos

Tudo o que o Studio sabe está aqui. Nada em banco de dados, nada em nuvem.

```text
meu-projeto/
├── .arch/
│   ├── manifest.yaml              # metadados do projeto e preferências
│   ├── pricing.yaml               # valor/hora, horas base, catálogo de nuvem
│   ├── tasks.json                 # espelho legível por máquina do ai-prd.md
│   ├── document.yaml              # cliente, usuários, histórico e referências do documento de requisitos
│   └── diagrams/
│       ├── macro.json             # nós, arestas, posições — a fonte da verdade visual
│       ├── macro.mermaid          # espelho Mermaid, regenerado a cada gravação
│       ├── usecase/               # diagramas UML de casos de uso (<id>.json + <id>.mermaid)
│       ├── class/                 # diagramas UML de classes
│       ├── sequence/              # diagramas UML de sequência
│       ├── state/                 # diagramas UML de estados
│       └── er/                    # modelagem de entidades
├── docs/
│   ├── requisitos.md              # RFs e RNFs em formato estruturado e estável
│   ├── ai-prd.md                  # blueprint de implementação para agentes de IA
│   ├── proposta-comercial.md      # proposta gerada a partir do escopo visual
│   ├── documento-de-requisitos.md # documento de requisitos formal, gerado
│   ├── diagramas/                 # SVGs das figuras do documento de requisitos
│   ├── casos-de-uso/              # um arquivo por caso de uso
│   └── architecture-decisions/    # ADRs no formato Nygard
└── api/
    ├── endpoints.yaml             # contratos derivados das arestas do diagrama
    └── openapi.yaml               # OpenAPI 3.1 exportado sob demanda
```

---

## Interface

O layout segue o StarUML: **barra de menus** (Arquivo, Editar, Exibir, Modelo, Ferramentas, Ajuda),
**Toolbox** à esquerda com as formas do diagrama aberto, **abas de diagramas** no centro,
**Model Explorer** (árvore do projeto) e **Editor** de propriedades à direita, e **barra de status**
com arquivo, contagens, qualidade e zoom. Os painéis são redimensionáveis e ocultáveis
(`Ctrl+B` / `Ctrl+J`). Os componentes de interface são do [shadcn/ui](https://ui.shadcn.com)
(Radix + Tailwind), com **tema claro, escuro ou do sistema** (menu *Exibir → Tema*), que vale também
para a área de desenho.

| Tela | O que faz |
| :--- | :--- |
| **Arquitetura** | Canvas do diagrama macro em notação de componentes UML («service», «database»…), com 9 tipos de componente, conexões tipadas, agrupamentos, minimapa e editor de metadados. Arrastar um nó grava em disco imediatamente. |
| **Diagramas UML** | Casos de uso, classes, sequência e estados (veja abaixo). Escolha a forma na Toolbox e clique no canvas; relações por arraste entre as alças ou por clique na origem e no destino. Exportação em PNG, SVG e Mermaid. |
| **Documentação** | Requisitos (RF/RNF), casos de uso com fluxos e critérios Given-When-Then, ADRs, visualização do AI-PRD e da proposta comercial. |
| **Contratos** | Tabela editável de `api/endpoints.yaml` com exportação para OpenAPI 3.1. |
| **Precificação** | Esforço por camada, distribuição por perfil, custo de nuvem, prazo e editor da tabela de preços. |
| **Implementação** | Fila de tarefas em ordem topológica, com dependências, critérios de aceite e progresso — a mesma que a IA consome. |
| **Pitch** | Apresentação em tela cheia com zoom cinematográfico por componente, alternância negócio/engenharia e exportação em SVG, PNG e PDF. |

### Edição no canvas

Como no StarUML, dá para modelar sem voltar à Toolbox:

- **Clique direito** no vazio: *Adicionar aqui* (qualquer forma do diagrama), colar, selecionar tudo.
  Em um elemento: *Adicionar conectado* (subclasse, interface realizada, caso incluído, próximo estado,
  mensagem para nova linha de vida…), *Adicionar dentro* (pacotes, fronteiras, estados compostos),
  atributos/operações, renomear, copiar, recortar, duplicar e excluir.
- **Barra rápida**: ao selecionar um elemento, aparece acima dele uma barra com os atalhos de
  *Adicionar conectado* — um clique cria o elemento já ligado e posicionado.
- **Duplo clique** no vazio abre *Adicionar aqui* na posição do cursor; em um elemento, renomeia.
- **Seleção múltipla** por caixa (arrastar no vazio), `Ctrl`/`Shift`+clique ou `Ctrl+A`; `Del` exclui
  tudo de uma vez, sem diálogo de confirmação — a exclusão é desfazível.

| Atalho | Ação |
| :--- | :--- |
| `Ctrl+Z` / `Ctrl+Y` (ou `Ctrl+Shift+Z`) | Desfazer / refazer — vale para qualquer mudança no diagrama, inclusive as feitas por IA |
| `Ctrl+C` / `Ctrl+X` / `Ctrl+V` | Copiar, recortar e colar (as relações entre os elementos copiados vão junto) |
| `Ctrl+D` | Duplicar a seleção |
| `Ctrl+A` | Selecionar tudo |
| `Del` / `Backspace` | Excluir a seleção |
| `F2` | Renomear o elemento selecionado |
| `Esc` | Voltar à ferramenta Selecionar / cancelar relação |
| `Shift+1`, `+`, `−` | Ajustar à tela, aproximar, afastar |

Cada operação de edição grava o diagrama inteiro num único passo, então um `Ctrl+Z` desfaz a operação
toda (por exemplo, colar 5 elementos). Excluir arquivos — diagramas, fichas, requisitos, ADRs e
contratos — pede confirmação num diálogo, pois isso não entra no histórico.

No Modo Pitch: `→`/`←` navega, `E` alterna executivo/engenharia, `F` tela cheia, `Esc` sai.

---

## Diagramas UML

Além do diagrama macro de componentes, cada projeto guarda diagramas UML de quatro tipos, um arquivo
JSON por diagrama (mais o espelho `.mermaid`, quando `sync_mermaid` está ativo):

| Tipo | Pasta | Elementos | Relações |
| :--- | :--- | :--- | :--- |
| Casos de uso | `.arch/diagrams/usecase/` | ator, caso de uso, fronteira, nota | associação, include, extend, generalização, dependência |
| Classes | `.arch/diagrams/class/` | classe, interface, enum, pacote, nota | associação (dirigida ou não), agregação, composição, generalização, realização, dependência |
| Sequência | `.arch/diagrams/sequence/` | lifeline, fragmento combinado, nota | mensagem (sync, async, reply, create, destroy) |
| Estados | `.arch/diagrams/state/` | estado (simples ou composto), inicial, final, escolha, fork, join, histórico, nota | transição (`evento [guarda] / efeito`) |

O `id` do diagrama é o slug do nome na criação e nunca muda. Setas e losangos ficam sempre na ponta
de destino (filho → pai, parte → todo, origem → destino), e mensagens de sequência têm ordem única
1..n, renumerada a cada inclusão ou remoção. Classes, lifelines e estados podem apontar para um
componente do diagrama macro via `component_id`, e casos de uso para a ficha via `use_case`.

O diagrama de casos de uso pode ser gerado a partir das fichas de `docs/casos-de-uso/` — de forma
idempotente, preservando posições. Todos os diagramas entram no `docs/ai-prd.md` (seção *UML MODELS*)
como blocos Mermaid e fazem parte do hash de integridade. O `init` sem `--empty` cria um exemplo de
cada tipo, coerente com o caso de uso de autenticação.

---

## Documento de requisitos

O Studio gera o **Documento de Requisitos** formal do projeto — capa, histórico de alterações,
sumário, introdução, descrição geral, requisitos funcionais, não funcionais agrupados por categoria
(com o quadro de prioridade Essencial/Importante/Desejável), diagramas de casos de uso, detalhamento
de cada CDU, modelagem UML, arquitetura, matriz de rastreabilidade e referências. Seções sem conteúdo
são omitidas e a numeração de títulos e figuras se ajusta sozinha.

Nada é digitado duas vezes: o documento é montado a partir de `docs/requisitos.md` (use
`- **Categoria:**` e `- **Requisitos associados:**` nos RNFs), das fichas de `docs/casos-de-uso/`
(descrição, `- **Requisitos:**`, pós-condições e subtítulos nos fluxos — um item `# Cadastro` agrupa
passos sem interromper a numeração), dos diagramas UML, do diagrama macro, dos ADRs e dos contratos.
Só o que não existe em outro lugar — cliente, usuários, histórico, referências e glossário — fica em
`.arch/document.yaml`. As figuras são SVGs renderizados no servidor, idênticos ao canvas.

| Onde | Como |
| :--- | :--- |
| **Interface** | Pré-visualização do documento, edição dos metadados e exportação em PDF, DOCX e Markdown. |
| **CLI** | `archcode-studio export requirements [--out docs/documento-de-requisitos.md]` grava o Markdown e as figuras em `docs/diagramas/`. |
| **MCP** | `update_document_metadata` preenche cliente, usuários e histórico; `generate_requirements_document` grava o documento e devolve um resumo. |
| **HTTP** | `GET /api/reqdoc` (estruturado), `GET /api/reqdoc/markdown?embed=1` (arquivo único), `POST /api/reqdoc/save`, `GET /api/export/uml/<id>.svg`. |

---

## Integração com agentes de IA (MCP)

```bash
# Claude Code
claude mcp add archcode-studio -- archcode-studio mcp --dir "$PWD"

# Cursor, Antigravity, Windsurf, Roo Code — .mcp.json do projeto
{
  "mcpServers": {
    "archcode-studio": {
      "command": "archcode-studio",
      "args": ["mcp", "--dir", "/caminho/do/projeto"]
    }
  }
}

# Transporte SSE, com `archcode-studio serve` rodando
http://127.0.0.1:8765/mcp/sse
```

### Ferramentas expostas

| Ferramenta | O que faz |
| :--- | :--- |
| `get_system_context` | Visão macro: componentes, conexões, endpoints, requisitos e progresso. |
| `get_architecture_summary` | Resumo ultracompacto em texto, para janelas de contexto apertadas. |
| `get_full_context` | Estado consolidado completo do projeto. |
| `add_architecture_node` | Cria um componente calculando posição livre, sem sobrepor nós existentes. |
| `connect_nodes` | Conecta componentes com protocolo, porta, segurança e contratos HTTP. |
| `update_node_metadata` | Altera tecnologia, descrição, tags e precificação — preservando a posição. |
| `remove_architecture_node` | Remove componente, conexões e contratos órfãos. |
| `upsert_requirement` | Cria ou atualiza RF/RNF em `docs/requisitos.md`. |
| `upsert_use_case` | Cria ou atualiza um caso de uso estruturado. |
| `upsert_adr` | Registra uma decisão arquitetural. |
| `calculate_project_estimate` | Horas, custo por perfil, infraestrutura e prazo. |
| `generate_commercial_proposal` | Gera a proposta comercial completa. |
| `generate_ai_prd` | Compila o blueprint com ordem topológica e critérios de aceite. |
| `get_implementation_tasks` | Fila de tarefas com o campo `ready` (dependências satisfeitas). |
| `mark_task_status` | Registra progresso — reflete no canvas do usuário em tempo real. |
| `validate_architecture_rules` | Linter de 12 regras arquiteturais. |
| `import_mermaid_diagram` | Importa um snippet Mermaid preservando coordenadas conhecidas. |
| `export_openapi` | Gera `api/openapi.yaml`. |
| `list_uml_diagrams` | Lista os diagramas UML com contadores de elementos e relações. |
| `get_uml_diagram` | JSON compacto (sem coordenadas) de um diagrama UML, mais o Mermaid. |
| `create_uml_diagram` | Cria um diagrama de casos de uso, classes, sequência ou estados. |
| `add_uml_element` | Adiciona ator, classe, lifeline, estado… numa posição livre. Aceita membros em notação UML (`+ login(email: string): Token`). |
| `update_uml_element` | Atualiza campos de um elemento (merge), preservando a posição. |
| `add_uml_relation` | Liga elementos por id ou nome: herança, composição, mensagem, transição… |
| `remove_uml_item` | Remove um elemento (com suas relações) ou uma relação; mensagens são renumeradas. |
| `generate_use_case_diagram` | Gera ou completa o diagrama de casos de uso a partir de `docs/casos-de-uso`. |
| `update_document_metadata` | Atualiza (merge) cliente, usuários, histórico, referências e glossário do documento de requisitos. |
| `generate_requirements_document` | Grava `docs/documento-de-requisitos.md` e as figuras; devolve o número de RF, RNF, CDU e figuras. |

Dois prompts acompanham o servidor: `implement_next_task` e `model_new_feature`.

### O ciclo completo

```
Usuário: "Crie um microserviço de pagamentos conectado ao Postgres e ao Stripe"
   ↓
IA → get_system_context          entende o que já existe
IA → add_architecture_node       cria o serviço numa posição livre
IA → connect_nodes               liga ao banco e ao Stripe, declara as rotas
IA → upsert_requirement          registra o RF que justifica o componente
IA → validate_architecture_rules corrige o que vier como "error"
IA → generate_ai_prd             compila o blueprint de implementação
   ↓
Canvas do usuário atualiza em <50ms, com animação no nó recém-criado
   ↓
IA → get_implementation_tasks    pega a primeira tarefa pronta
IA → escreve o código, roda os testes
IA → mark_task_status            o nó fica verde no canvas
```

---

## CLI

```bash
archcode-studio init        [--name --description --author --currency --empty --force]
archcode-studio serve       [--port 8765 --host 127.0.0.1 --open --no-mcp]
archcode-studio mcp         [--dir .]                  # servidor MCP em stdio
archcode-studio prd         [--stack --granularity --no-tests]
archcode-studio estimate    [--margin --json --detailed]
archcode-studio validate    [--json --strict]          # sai com 1 se houver erros
archcode-studio export      svg|mermaid|openapi|proposal|requirements [--mode --theme --out]
archcode-studio mcp-config
```

`validate --strict` no CI reprova o build quando a arquitetura regride.

---

## Como funciona

### Preservação de coordenadas

Nenhuma operação de agente move um nó já posicionado. Componentes novos recebem posição calculada em
anéis crescentes ao redor do nó âncora (aquele ao qual serão conectados), testando colisão contra
todos os nós existentes. Só o arrasto humano no canvas altera posições — e o `AutoLayout` por camadas
topológicas, que exige confirmação explícita.

### Sincronização em tempo real

O `fsnotify` observa `.arch/`, `docs/` e `api/` com *debounce* de 300 ms. Gravações feitas pelo
próprio servidor são filtradas por um índice de eco (hash SHA-256 da última escrita), de modo que
apenas edições externas — VS Code, `git checkout`, CLI de IA — chegam ao navegador. Toda escrita é
atômica (arquivo temporário + `rename`) e serializada por um mutex de transação, então chamadas MCP
concorrentes nunca se sobrescrevem.

### Compilador de AI-PRD

O `generate_ai_prd` percorre o grafo de dependências (`A → B` significa "A consome B", logo B é
pré-requisito de A) e emite tarefas em ordem topológica: banco → domínio → serviços → integrações →
interface → testes E2E. Ciclos são resolvidos pelo rank da camada, sem travar. Cada tarefa carrega
dependências, requisitos, casos de uso, contratos e critérios de aceite verificáveis. Regenerar o
documento **preserva o progresso já registrado** pelos agentes.

### Precificação

```
Horas Totais = (Σ Horas(Nós) + Σ Horas(Arestas) + Σ Horas(Casos de Uso)) × (1 + Margem)
```

Cada parcela aceita valor explícito; na ausência dele, aplica-se o catálogo de `.arch/pricing.yaml`
(horas base por tipo de componente × fator de complexidade). O resultado é distribuído por perfil
profissional, somado a impostos, e acrescido do custo mensal de nuvem por tier de instância.

---

## Desenvolvimento

```bash
make deps        # dependências de Go e do frontend
make dev         # backend :8765 + Vite com hot reload :5173
make check       # gofmt, go vet, testes e typecheck — o mesmo que a CI roda
make test-race   # testes com o detector de corrida
make build       # binário único com o frontend embutido
make release     # binários para Linux, macOS (Intel e Apple Silicon) e Windows
```

### Organização do código

```text
cmd/archcode-studio/    CLI e composição dos subcomandos
internal/
  model/                estruturas canônicas + serialização Markdown bidirecional
  store/                acesso ao disco: escrita atômica, supressão de eco, snapshot
  app/                  operações de domínio — única camada que muta estado
  layout/               posicionamento inteligente e layout topológico
  mermaid/              exportação determinística e importação de snippets
  prd/                  compilador do AI-PRD e ordenação topológica
  pricing/              motor de esforço, custo e prazo
  lint/                 linter de regras arquiteturais
  svgexport/            renderização SVG server-side
  httpapi/              REST + WebSocket + SPA embutida
  mcpserver/            ferramentas e prompts MCP
  hub/ watcher/         eventos em tempo real e observação do disco
  project/              scaffolding do `init`
  webui/                embed.FS do bundle compilado
web/                    frontend React 19 + Vite + Tailwind 4 + shadcn/ui + React Flow 12
  src/components/ui/    componentes shadcn/ui (components.json na raiz de web/)
  src/components/shell/ menus, Toolbox e Model Explorer (layout estilo StarUML)
  src/components/uml/   formas, relações, canvas e editor dos diagramas UML
```

A API HTTP e o servidor MCP chamam **exatamente os mesmos métodos** do pacote `app`. Uma escrita de
IA passa pelas mesmas invariantes de uma ação humana — é isso que torna a colaboração segura.

### Preparação para Wails v3

O frontend conversa com o backend apenas através de `web/src/lib/transport.ts`. Para empacotar como
app desktop nativo, registre ali uma implementação de IPC: nenhum componente de interface muda.

---

## Decisões de implementação que divergem do PRD original

Duas escolhas conscientes, ambas documentadas no código:

1. **Editor de requisitos estruturado em vez de BlockNote.** A serialização Markdown de editores
   WYSIWYG genéricos é lossy e reescreveria o arquivo inteiro a cada edição, quebrando o formato
   determinístico de que o parser Go e os diffs de Git dependem (RNF001). A tela entrega edição em
   blocos de verdade — um formulário por requisito — mais um editor Markdown bruto para quem prefere.

2. **Agrupamentos com coordenadas absolutas.** Nós de grupo (VPC, cluster, contexto delimitado) são
   nós comuns renderizados atrás dos demais, em vez de usar o sistema de nó-pai do React Flow, que
   exige coordenadas relativas. Isso mantém `macro.json` legível e todas as posições comparáveis
   diretamente — essencial para a regra de preservação de coordenadas.

---

## Roadmap

- [x] **Fase 1** — Core Go, WebSocket, file watcher, canvas React Flow
- [x] **Fase 2** — Servidor MCP (stdio + SSE), ferramentas atômicas, compilador de AI-PRD
- [x] **Fase 3** — Documentação, Mermaid bidirecional, Modo Pitch, exportações
- [x] **Fase 4** — Precificação, proposta comercial, linter, OpenAPI 3.1
- [ ] **Fase 5** — App desktop nativo com Wails v3 (a camada de transporte já está pronta)

---

## Licença

MIT.
