# ArchCode Studio 🏗️

[![Release](https://img.shields.io/github/v/release/VS-7/arch-studio?style=flat)](https://github.com/VS-7/arch-studio/releases/latest)
[![CI](https://github.com/VS-7/arch-studio/actions/workflows/ci.yml/badge.svg)](https://github.com/VS-7/arch-studio/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/Go-1.25-00ADD8?style=flat&logo=go)](https://go.dev)
[![React](https://img.shields.io/badge/React-19-61DAFB?style=flat&logo=react)](https://react.dev)
[![Wails](https://img.shields.io/badge/Wails-v3-DF0000?style=flat)](https://v3.wails.io)
[![MCP](https://img.shields.io/badge/MCP-stdio_%7C_SSE-6E56CF?style=flat)](https://modelcontextprotocol.io)
[![Docker](https://img.shields.io/badge/Docker-Ready-2496ED?style=flat&logo=docker)](https://www.docker.com)
[![Coolify](https://img.shields.io/badge/Coolify-Compatible-6B21A8?style=flat)](https://coolify.io)

**ArchCode Studio** é uma ferramenta de **arquitetura de software como código**: local-first, versionável em Git e nativa para agentes de IA. Projete, documente, valide, precifique e implemente arquiteturas com sincronização em tempo real entre **humanos** (canvas no estilo StarUML) e **agentes de IA** (Model Context Protocol) — um único binário, sem banco de dados, sem nuvem, sem lock-in.

```
┌─ Humano ──────────────┐   ┌─ Núcleo Go ─────────────┐   ┌─ .arch/ + docs/ + api/ ──┐
│ Canvas e diagramas UML│◄─►│ HTTP + WebSocket        │◄─►│ macro.json · UML         │
│ Documentação e PDF    │   │ Watcher do disco        │   │ requisitos.md            │
│ Precificação · Pitch  │   │ Compilador de AI-PRD    │   │ casos-de-uso/ · ADRs     │
└───────────────────────┘   │ Linter · Precificação   │   │ endpoints.yaml           │
┌─ Agente de IA ────────┐   │ Servidor MCP            │   │ ai-prd.md · tasks.json   │
│ Claude Code · Cursor  │◄─►└─────────────────────────┘   └──────────────────────────┘
│ Antigravity · Windsurf│        stdio · SSE                  Git versiona tudo ▲
└───────────────────────┘
```

---

## ⚡ Instalação

### CLI — um comando, já configurado no PATH

**Linux e macOS** (Intel e Apple Silicon):

```bash
curl -fsSL https://raw.githubusercontent.com/VS-7/arch-studio/main/scripts/install.sh | sh
```

**Windows** (PowerShell):

```powershell
irm https://raw.githubusercontent.com/VS-7/arch-studio/main/scripts/install.ps1 | iex
```

O instalador baixa o binário da [última release](https://github.com/VS-7/arch-studio/releases/latest) para o seu sistema, **confere o SHA-256**, instala sem sudo/administrador (`~/.local/bin` ou `%LOCALAPPDATA%\Programs\ArchCode Studio`) e adiciona a pasta ao PATH. Rodar de novo atualiza para a versão mais recente. Versão específica: `curl -fsSL …/install.sh | ARCHCODE_VERSION=1.0.0 sh`.

### App desktop

Baixe em [Releases](https://github.com/VS-7/arch-studio/releases/latest) — mesma interface, sem servidor nem navegador:

| Sistema | Arquivo | Instalação |
| :--- | :--- | :--- |
| Ubuntu 24.04+ / Debian 13+ | `archcode-studio_<versão>_amd64.deb` | `sudo apt install ./archcode-studio_*_amd64.deb` (já instala GTK 4 e WebKitGTK 6.0) |
| Fedora 40+ | `archcode-studio-<versão>-1.x86_64.rpm` | `sudo dnf install ./archcode-studio-*.rpm` |
| Outras distros | `archcode-desktop-<versão>-linux-amd64.tar.gz` | extraia e rode `./archcode-desktop` (requer GTK 4.14+ e WebKitGTK 6.0) |
| Windows 10/11 | `archcode-desktop-<versão>-windows-amd64.zip` | extraia e rode o `.exe` |

### Docker

```bash
git clone https://github.com/VS-7/arch-studio.git && cd arch-studio
cp .env.example .env
docker compose up -d --build     # http://localhost:8765
```

---

## 🚀 Primeiros Passos

```bash
mkdir meu-projeto && cd meu-projeto
archcode-studio init --name "E-Commerce" --author "Seu Nome"   # estrutura + arquitetura de exemplo
archcode-studio serve                                          # abre http://127.0.0.1:8765
claude mcp add archcode-studio -- archcode-studio mcp --dir "$PWD"
```

O `init` cria uma arquitetura de exemplo funcional (Web App → Gateway → API → PostgreSQL/Redis) com requisitos, caso de uso, ADR e um diagrama UML de cada tipo. Use `--empty` para começar do zero. No app desktop, o mesmo fluxo está em **Arquivo → Novo projeto…**.

---

## ✨ Principais Funcionalidades

1. **Canvas de arquitetura no estilo StarUML:**
   - Toolbox, abas de diagramas, Model Explorer e Editor de propriedades — painéis redimensionáveis, recolhíveis e maximizáveis; tema claro, escuro ou do sistema.
   - 9 tipos de componente, conexões com protocolo/porta/segurança, agrupamentos e minimapa. Menu de contexto, barra rápida de *Adicionar conectado*, seleção múltipla, copiar/colar e `Ctrl+Z` para qualquer mudança — inclusive as feitas por IA.
   - **Preservação de coordenadas:** nenhuma operação de agente move um nó já posicionado; componentes novos entram numa posição livre ao redor do nó ao qual se conectam.

2. **Diagramas UML:**
   - Casos de uso, classes, sequência e estados — um JSON por diagrama em `.arch/diagrams/<tipo>/`, com espelho Mermaid e exportação PNG/SVG.
   - Diagrama de casos de uso gerado a partir das fichas de `docs/casos-de-uso/`, de forma idempotente.
   - **Reorganizar** (menu **Modelo**, `Ctrl+Shift+L` ou o botão do Documento de Requisitos) redispõe o diagrama ativo — ou todos de uma vez — com espaçamento entre os elementos e na proporção da página A4, para as figuras do documento ficarem legíveis: casos de uso em grade com os atores nas laterais, classes e estados em camadas, sequência espaçada pelos rótulos das mensagens. `Ctrl+Z` (ou o *Desfazer* do aviso) volta atrás.

3. **Documentação que se escreve sozinha:**
   - Requisitos (RF/RNF), casos de uso com fluxos e critérios Given-When-Then, ADRs no formato Nygard — cada um na sua aba.
   - **Documento de Requisitos formal** (capa, sumário, requisitos, CDUs, diagramas, rastreabilidade) montado a partir do projeto e exportado em **PDF, DOCX e Markdown**.

4. **Nativo para agentes de IA (MCP):**
   - 29 ferramentas atômicas via stdio ou SSE, com as mesmas regras de uma edição humana.
   - **Compilador de AI-PRD:** ordena as tarefas topologicamente (banco → domínio → serviços → integrações → interface → E2E), com dependências e critérios de aceite; os agentes marcam o progresso e o canvas acompanha em tempo real.

5. **Precificação e proposta comercial:**
   - Esforço por componente, integração e caso de uso, com margem de risco, distribuição por perfil, impostos e custo mensal de nuvem.
   - Proposta técnica e comercial gerada a partir do escopo modelado; linter com 12 regras arquiteturais e exportação OpenAPI 3.1.

6. **Local-first e amigável ao Git:**
   - Tudo em arquivos de texto na árvore do projeto; gravações atômicas e serializadas.
   - Edições externas (VS Code, `git checkout`, agentes em outro processo) aparecem na interface em tempo real.

7. **Três formas de usar, o mesmo núcleo:**
   - **CLI + navegador** (`archcode-studio serve`), **app desktop** (Wails v3, com diálogos nativos de abrir/salvar/imprimir) e **Docker/Coolify** para uso em equipe.
   - **Zero Node.js em produção:** o frontend é embutido no executável via `go:embed`.

---

## 🤖 Integração com Agentes de IA (MCP)

```bash
# Claude Code
claude mcp add archcode-studio -- archcode-studio mcp --dir "$PWD"
```

Cursor, Antigravity, Windsurf, Roo Code — no `.mcp.json` do projeto:

```json
{ "mcpServers": { "archcode-studio": { "command": "archcode-studio", "args": ["mcp", "--dir", "/caminho/do/projeto"] } } }
```

Com `archcode-studio serve` rodando, o transporte SSE fica em `http://127.0.0.1:8765/mcp/sse`. No app desktop, o próprio executável serve MCP (`archcode-desktop mcp --dir <projeto>`), e **Ferramentas → Conectar agente de IA** mostra o comando pronto.

| Área | Ferramentas |
| :--- | :--- |
| **Contexto** | `get_system_context` · `get_architecture_summary` · `get_full_context` |
| **Arquitetura** | `add_architecture_node` · `connect_nodes` · `update_node_metadata` · `remove_architecture_node` · `import_mermaid_diagram` · `validate_architecture_rules` |
| **UML** | `list_uml_diagrams` · `get_uml_diagram` · `create_uml_diagram` · `add_uml_element` · `update_uml_element` · `add_uml_relation` · `remove_uml_item` · `generate_use_case_diagram` · `auto_layout_diagram` |
| **Documentação** | `upsert_requirement` · `upsert_use_case` · `upsert_adr` · `update_document_metadata` · `generate_requirements_document` |
| **Implementação** | `generate_ai_prd` · `get_implementation_tasks` · `mark_task_status` |
| **Negócio** | `calculate_project_estimate` · `generate_commercial_proposal` · `export_openapi` |

Prompts: `implement_next_task` e `model_new_feature`. O ciclo típico:

```
IA → get_system_context → add_architecture_node → connect_nodes → upsert_requirement
   → validate_architecture_rules → generate_ai_prd
   → get_implementation_tasks → implementa e testa → mark_task_status   (o nó fica verde no canvas)
```

---

## ⌨️ CLI

| Comando | O que faz |
| :--- | :--- |
| `archcode-studio init [--name --author --empty]` | Cria `.arch/`, `docs/` e `api/` (com exemplo, por padrão) |
| `archcode-studio serve [--port 8765 --host --no-mcp]` | Interface web + WebSocket + MCP via SSE |
| `archcode-studio mcp [--dir]` | Servidor MCP em stdio para agentes de IA |
| `archcode-studio prd [--stack --granularity --no-tests]` | Compila `docs/ai-prd.md` e `.arch/tasks.json` |
| `archcode-studio estimate [--margin --json --detailed]` | Esforço, custo e prazo |
| `archcode-studio validate [--json --strict]` | Linter de arquitetura — sai com código 1 se houver erros (ótimo na CI) |
| `archcode-studio export svg\|mermaid\|openapi\|proposal\|requirements` | Exportações (`--mode`, `--theme`, `--out`) |
| `archcode-studio mcp-config` | Imprime a configuração MCP para o projeto |

---

## 🗂️ Estrutura de um Projeto

Tudo o que o Studio sabe fica na árvore do seu projeto — nada em banco de dados, nada em nuvem:

```
meu-projeto/
├── .arch/
│   ├── manifest.yaml              # Metadados do projeto e preferências
│   ├── pricing.yaml               # Valor/hora, horas base e catálogo de nuvem
│   ├── document.yaml              # Cliente, usuários e histórico do Documento de Requisitos
│   ├── tasks.json                 # Espelho legível por máquina do ai-prd.md
│   └── diagrams/
│       ├── macro.json             # Nós, arestas e posições — a fonte da verdade visual
│       ├── macro.mermaid          # Espelho Mermaid, regenerado a cada gravação
│       └── usecase/ class/ sequence/ state/   # Diagramas UML
├── docs/
│   ├── requisitos.md              # RFs e RNFs em formato estruturado
│   ├── casos-de-uso/              # Um arquivo por caso de uso
│   ├── architecture-decisions/    # ADRs
│   ├── ai-prd.md                  # Blueprint de implementação para agentes de IA
│   ├── proposta-comercial.md      # Proposta gerada a partir do escopo
│   └── documento-de-requisitos.md # Documento formal + figuras em diagramas/
└── api/
    ├── endpoints.yaml             # Contratos derivados das conexões do diagrama
    └── openapi.yaml               # OpenAPI 3.1 exportado sob demanda
```

---

## 🏛️ Arquitetura e Estrutura do Código

Camadas com **adaptadores finos** sobre um único conjunto de casos de uso, seguindo **SOLID** onde traz ganho real: HTTP, MCP, CLI e app desktop chamam exatamente os mesmos métodos do pacote `app`, montados num só lugar (`internal/studio`). Uma escrita de IA passa pelas mesmas invariantes de uma ação humana. O `app` publica eventos num barramento em memória e cada transporte só assina — WebSocket no navegador, eventos do Wails no desktop.

```
arch-studio/
├── cmd/
│   └── archcode-studio/          # CLI: um arquivo por comando (serve, mcp, init, relatórios, export)
├── desktop/                      # App desktop Wails v3 (módulo Go próprio, com CGO)
│   ├── internal/workspace/       # Projeto aberto, recentes e handler da janela (testável sem GUI)
│   └── build/linux/              # .desktop e pacotes .deb/.rpm (nfpm)
├── internal/
│   ├── studio/                   # Composition Root: store + app + watcher + HTTP + MCP de uma pasta
│   ├── app/                      # Casos de uso — única camada que lê e muta o projeto
│   ├── model/                    # Estruturas canônicas, serialização Markdown e tipos de erro
│   ├── store/                    # Disco: escrita atômica, supressão de eco, snapshot
│   ├── httpapi/                  # Adaptador REST + WebSocket + SPA embutida
│   ├── mcpserver/                # Adaptador MCP: ferramentas e prompts
│   ├── hub/                      # Barramento de eventos em memória
│   ├── watcher/                  # Edições externas no disco (fsnotify)
│   ├── prd/  proposal/  openapi/  reqdoc/        # Geradores puros (entrada → saída),
│   ├── lint/  pricing/  layout/  mermaid/  svgexport/   # testados isoladamente
│   ├── project/                  # Scaffolding do init
│   └── webui/                    # Frontend compilado embutido (go:embed)
├── scripts/                      # Instaladores do CLI (install.sh, install.ps1)
└── web/                          # React 19 + TypeScript + Vite + Tailwind 4 + shadcn/ui + React Flow 12
    └── src/lib/                  # transport.ts (HTTP + eventos) e platform.ts (salvar, imprimir, copiar)
```

---

## ☁️ Deploy no Coolify

O repositório já está pronto para deploy (`Dockerfile` + `docker-compose.yml`):

1. Conecte o repositório Git no Coolify (branch `main`).
2. Selecione o tipo de build **Docker Compose**.
3. Volume persistente: `/workspace` (onde ficam `.arch/`, `docs/` e `api/`).
4. Configure as variáveis com base no `.env.example` — defina `ARCHCODE_BASE_URL` com o domínio público para o MCP via SSE.
5. Inicie o deploy! No primeiro boot, o container roda `init` automaticamente; o healthcheck usa `GET /api/health`.

> ⚠️ O servidor não tem autenticação própria. Em instância pública, proteja o domínio (ex.: Basic Auth via labels do Traefik no Coolify) ou restrinja o acesso por rede.

---

## ⚙️ Variáveis de Ambiente

**Servidor e Docker Compose:**

| Variável | Padrão | Descrição |
| :--- | :--- | :--- |
| `PORT_BIND` | `8765` | Porta exposta no host via Docker Compose |
| `WORKSPACE_DIR_HOST` | `./workspace` | Pasta do host com o projeto (`.arch/`, `docs/`, `api/`) |
| `ARCHCODE_PROJECT_NAME` | `ArchCode Project` | Nome do projeto criado no primeiro boot |
| `ARCHCODE_BASE_URL` | — | URL pública anunciada pelo MCP via SSE (atrás de proxy reverso) |
| `PORT` | `8765` | Porta interna do servidor no container |
| `ARCHCODE_DIR` | `/workspace` | Pasta do projeto dentro do container |

**Instaladores do CLI:**

| Variável | Padrão | Descrição |
| :--- | :--- | :--- |
| `ARCHCODE_VERSION` | última release | Versão a instalar (ex.: `1.0.0`) |
| `ARCHCODE_INSTALL_DIR` | `~/.local/bin` · `%LOCALAPPDATA%\Programs\ArchCode Studio` | Pasta do executável |
| `ARCHCODE_NO_MODIFY_PATH` | — | Linux/macOS: não altera o arquivo de configuração do shell |

Para desinstalar: `rm ~/.local/bin/archcode-studio` (Linux/macOS) ou `Remove-Item -Recurse "$env:LOCALAPPDATA\Programs\ArchCode Studio"` (Windows).

---

## 🧪 Desenvolvimento e Testes

Requer Go 1.25+ e Node 20+.

```bash
make deps              # Dependências de Go e do frontend
make dev               # Backend :8765 + Vite com hot reload :5173
make check             # gofmt, go vet, testes, typecheck e testes do desktop — o mesmo que a CI
make test-race         # Testes com o detector de race condition
make build             # Binário único com o frontend embutido
make release           # CLI para Linux, macOS (Intel e Apple Silicon) e Windows em dist/

make desktop           # App desktop para o sistema atual (Linux: libgtk-4-dev libwebkitgtk-6.0-dev)
make desktop-windows   # App desktop para Windows (compila de qualquer sistema, sem CGO)
make desktop-package   # Pacotes Linux .deb, .rpm e .tar.gz em dist/
make desktop-dev       # App desktop em modo desenvolvimento, com DevTools
```

**Releases:** cada tag `v*` (`git tag v1.1.0 && git push origin v1.1.0`) faz a CI publicar o CLI de todas as plataformas, os pacotes do app desktop e o `SHA256SUMS` — é deles que os instaladores baixam.

---

## 📄 Licença

Distribuído sob a licença MIT. Consulte `LICENSE` para obter mais detalhes.
