// Package project cria a estrutura canônica de um projeto ArchCode Studio.
package project

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/archcode/studio/internal/mermaid"
	"github.com/archcode/studio/internal/model"
	"github.com/archcode/studio/internal/store"
)

type Options struct {
	ProjectName string
	Description string
	Author      string
	Currency    string
	Empty       bool // não cria a arquitetura de exemplo
	Force       bool // sobrescreve um projeto existente
}

type Result struct {
	Root    string
	Created []string
	Skipped []string
}

// Init cria (ou completa) a árvore .arch/, docs/ e api/ na raiz informada.
func Init(st *store.Store, opts Options) (*Result, error) {
	if st.IsProject() && !opts.Force {
		return nil, fmt.Errorf("já existe um projeto em %s (use --force para sobrescrever o manifest)", st.Root())
	}

	name := strings.TrimSpace(opts.ProjectName)
	if name == "" {
		name = filepath.Base(st.Root())
	}
	currency := strings.ToUpper(strings.TrimSpace(opts.Currency))
	if currency == "" {
		currency = "BRL"
	}

	res := &Result{Root: st.Root()}

	for _, dir := range []string{
		store.DirArch, store.DirDiagrams, store.DirSequence, store.DirER,
		store.DirDocs, store.DirUseCases, store.DirADR, store.DirAPI,
	} {
		abs, err := st.Path(dir)
		if err != nil {
			return nil, err
		}
		if err := os.MkdirAll(abs, 0o755); err != nil {
			return nil, err
		}
	}

	write := func(rel string, data []byte, overwrite bool) error {
		if st.Exists(rel) && !overwrite {
			res.Skipped = append(res.Skipped, rel)
			return nil
		}
		if err := st.WriteFile(rel, data); err != nil {
			return err
		}
		res.Created = append(res.Created, rel)
		return nil
	}

	// manifest.yaml
	manifest := model.DefaultManifest(name)
	if opts.Description != "" {
		manifest.Description = opts.Description
	}
	if opts.Author != "" {
		manifest.Authors = []model.Author{{Name: opts.Author, Role: "Lead Architect"}}
	}
	manifest.Settings.PricingCurrency = currency
	if err := st.SaveManifest(manifest); err != nil {
		return nil, err
	}
	res.Created = append(res.Created, store.FileManifest)

	// pricing.yaml
	if !st.Exists(store.FilePricing) {
		pricing := model.DefaultPricing()
		pricing.Currency = currency
		if err := st.SavePricing(pricing); err != nil {
			return nil, err
		}
		res.Created = append(res.Created, store.FilePricing)
	} else {
		res.Skipped = append(res.Skipped, store.FilePricing)
	}

	// Diagrama macro.
	if !st.Exists(store.FileMacroJSON) {
		diagram := model.NewDiagram()
		endpoints := model.NewEndpointsSpec()
		if !opts.Empty {
			diagram, endpoints = starterArchitecture()
		}
		if err := st.SaveDiagram(diagram); err != nil {
			return nil, err
		}
		res.Created = append(res.Created, store.FileMacroJSON)
		if err := write(store.FileMacroMmd, []byte(mermaid.Export(diagram)), true); err != nil {
			return nil, err
		}
		if !st.Exists(store.FileEndpoints) {
			if err := st.SaveEndpoints(endpoints); err != nil {
				return nil, err
			}
			res.Created = append(res.Created, store.FileEndpoints)
		}
	} else {
		res.Skipped = append(res.Skipped, store.FileMacroJSON)
	}

	if !st.Exists(store.FileEndpoints) {
		if err := st.SaveEndpoints(model.NewEndpointsSpec()); err != nil {
			return nil, err
		}
		res.Created = append(res.Created, store.FileEndpoints)
	}

	// Requisitos.
	if !st.Exists(store.FileRequisit) {
		doc := &model.RequirementsDoc{
			ProjectName: name,
			Overview: strings.TrimSpace(`
Descreva aqui o objetivo do sistema, o problema de negócio que ele resolve e o público-alvo.
Este texto é lido pelos agentes de IA como contexto de produto antes de qualquer implementação.`),
		}
		if !opts.Empty {
			doc.Requirements = starterRequirements()
		}
		if err := st.SaveRequirements(doc); err != nil {
			return nil, err
		}
		res.Created = append(res.Created, store.FileRequisit)
	} else {
		res.Skipped = append(res.Skipped, store.FileRequisit)
	}

	// Template de caso de uso.
	if err := write(store.DirUseCases+"/_template.md", []byte(useCaseTemplate), false); err != nil {
		return nil, err
	}

	if !opts.Empty {
		uc := starterUseCase()
		if !st.Exists(store.DirUseCases + "/" + model.UseCaseFileName(uc)) {
			path, err := st.SaveUseCase(uc)
			if err != nil {
				return nil, err
			}
			res.Created = append(res.Created, path)
		}
		adr := starterADR()
		if _, err := st.SaveADR(adr); err == nil {
			res.Created = append(res.Created, adr.File)
		}
	}

	// .gitignore específico do estado transitório.
	if err := write(store.DirArch+"/.gitignore", []byte("*.tmp\n.archcode-*\n"), false); err != nil {
		return nil, err
	}
	for _, keep := range []string{store.DirSequence + "/.gitkeep", store.DirER + "/.gitkeep"} {
		if err := write(keep, []byte(""), false); err != nil {
			return nil, err
		}
	}

	return res, nil
}

// starterArchitecture cria uma arquitetura mínima e realista, para que o canvas
// não abra vazio e o usuário consiga explorar todas as funcionalidades de imediato.
func starterArchitecture() (*model.Diagram, *model.EndpointsSpec) {
	d := model.NewDiagram()
	yes := true
	no := false

	d.Nodes = []model.Node{
		{
			ID: "node-web-app", Type: "client", Position: model.Position{X: 120, Y: 200},
			Data: model.NodeData{
				Label: "Web App", Technology: "React 19 / Vite", Tier: "frontend",
				Description: "Interface principal utilizada pelos clientes finais.",
				Executive:   &yes, Status: model.StatusPending,
				Pricing: &model.NodePricing{Complexity: "medium", CloudTier: "vercel.pro"},
			},
		},
		{
			ID: "node-api-gateway", Type: "gateway", Position: model.Position{X: 440, Y: 200},
			Data: model.NodeData{
				Label: "API Gateway", Technology: "Traefik", Tier: "integration",
				Description: "Porta de entrada única: roteamento, TLS e rate limiting.",
				Executive:   &yes, Status: model.StatusPending,
				Pricing: &model.NodePricing{Complexity: "low", CloudTier: "cloud_run.small"},
			},
		},
		{
			ID: "node-core-api", Type: "compute", Position: model.Position{X: 760, Y: 200},
			Data: model.NodeData{
				Label: "Core API", Technology: "Go 1.23 / net/http", Tier: "backend",
				Description: "Regras de negócio e orquestração dos casos de uso.",
				Tags:        []string{"critical"},
				Executive:   &yes, Status: model.StatusPending,
				Pricing: &model.NodePricing{Complexity: "high", CloudTier: "ecs.fargate.0.5"},
			},
		},
		{
			ID: "node-postgres", Type: "database", Position: model.Position{X: 1080, Y: 120},
			Data: model.NodeData{
				Label: "PostgreSQL", Technology: "PostgreSQL 16", Tier: "data",
				Description: "Persistência transacional do domínio.",
				Executive:   &no, Status: model.StatusPending,
				Pricing: &model.NodePricing{Complexity: "medium", CloudTier: "db.t4g.small"},
			},
		},
		{
			ID: "node-redis", Type: "cache", Position: model.Position{X: 1080, Y: 300},
			Data: model.NodeData{
				Label: "Redis", Technology: "Redis 7", Tier: "data",
				Description: "Cache de sessões e rate limiting distribuído.",
				Executive:   &no, Status: model.StatusPending,
				Pricing: &model.NodePricing{Complexity: "low", CloudTier: "elasticache.t4g"},
			},
		},
	}

	d.Edges = []model.Edge{
		{
			ID: "edge-web-app-to-api-gateway", Source: "node-web-app", Target: "node-api-gateway",
			Type: "smoothstep",
			Data: model.EdgeData{Protocol: "REST", Port: 443, Security: "TLS + JWT",
				Description: "Chamadas da interface para a API", Complexity: "medium"},
		},
		{
			ID: "edge-api-gateway-to-core-api", Source: "node-api-gateway", Target: "node-core-api",
			Type: "smoothstep",
			Data: model.EdgeData{Protocol: "REST", Port: 8080, Security: "mTLS",
				Description: "Roteamento interno autenticado", Complexity: "low",
				Endpoints: []string{"post-api-v1-auth-login", "get-api-v1-me"}},
		},
		{
			ID: "edge-core-api-to-postgres", Source: "node-core-api", Target: "node-postgres",
			Type: "smoothstep",
			Data: model.EdgeData{Protocol: "SQL", Port: 5432,
				Description: "Leitura e escrita do domínio", Complexity: "medium"},
		},
		{
			ID: "edge-core-api-to-redis", Source: "node-core-api", Target: "node-redis",
			Type: "smoothstep",
			Data: model.EdgeData{Protocol: "Redis", Port: 6379,
				Description: "Cache de sessão", Complexity: "low"},
		},
	}

	spec := model.NewEndpointsSpec()
	spec.Endpoints = []model.Endpoint{
		{
			ID: "post-api-v1-auth-login", Method: "POST", Path: "/api/v1/auth/login",
			Summary: "Autentica o usuário e devolve um JWT",
			Source:  "node-api-gateway", Target: "node-core-api", EdgeID: "edge-api-gateway-to-core-api",
			Auth: "none", Request: `{"email": string, "password": string}`,
			Response:    `{"token": string, "expires_in": number}`,
			StatusCodes: []int{200, 401, 422},
			UseCases:    []string{"CDU001"},
		},
		{
			ID: "get-api-v1-me", Method: "GET", Path: "/api/v1/me",
			Summary: "Retorna o perfil do usuário autenticado",
			Source:  "node-api-gateway", Target: "node-core-api", EdgeID: "edge-api-gateway-to-core-api",
			Auth: "bearer", Response: `{"id": string, "email": string, "roles": string[]}`,
			StatusCodes: []int{200, 401},
			UseCases:    []string{"CDU001"},
		},
	}
	return d, spec
}

func starterRequirements() []model.Requirement {
	return []model.Requirement{
		{
			ID: "RF001", Type: "RF", Title: "Autenticação de usuários", Priority: "Alta",
			Status: model.StatusPending, Components: []string{"node-core-api", "node-postgres"},
			Description: "O sistema deve autenticar usuários por e-mail e senha, emitindo um token JWT com validade configurável.",
		},
		{
			ID: "RF002", Type: "RF", Title: "Consulta do perfil autenticado", Priority: "Média",
			Status: model.StatusPending, Components: []string{"node-core-api"},
			Description: "Usuários autenticados devem conseguir consultar seus próprios dados de perfil e papéis de acesso.",
		},
		{
			ID: "RNF001", Type: "RNF", Title: "Toda rota privada exige token JWT válido", Priority: "Alta",
			Status: model.StatusPending, Components: []string{"node-api-gateway", "node-core-api"},
			Description: "Requisições sem token ou com token expirado devem retornar HTTP 401 sem vazar detalhes internos.",
		},
		{
			ID: "RNF002", Type: "RNF", Title: "Tempo de resposta P95 abaixo de 300ms", Priority: "Média",
			Status: model.StatusPending, Components: []string{"node-core-api", "node-redis"},
			Description: "Endpoints de leitura devem responder em menos de 300ms no percentil 95 sob carga nominal.",
		},
	}
}

func starterUseCase() *model.UseCase {
	return &model.UseCase{
		Code: "CDU001", Name: "Autenticar Usuário",
		Actors: []string{"Usuário final"}, Components: []string{"node-web-app", "node-core-api", "node-postgres"},
		Complexity: "medium", EstimatedHours: 16, Priority: "Alta", Status: model.StatusPending,
		PreConditions: []string{"Usuário previamente cadastrado", "Conta ativa e não bloqueada"},
		MainFlow: []string{
			"O usuário informa e-mail e senha na tela de login",
			"A interface envia POST /api/v1/auth/login para a API",
			"A API valida as credenciais contra o hash armazenado no PostgreSQL",
			"A API emite um JWT assinado com validade de 1 hora",
			"A interface armazena o token e redireciona para o painel",
		},
		AlternateFlows: []string{"Usuário opta por login social: o fluxo delega ao provedor OAuth2 e retorna ao passo 4"},
		Exceptions: []string{
			"Credenciais inválidas: a API retorna 401 e a interface exibe mensagem genérica",
			"Cinco tentativas falhas em 10 minutos: a conta entra em bloqueio temporário",
		},
		BusinessRules: []string{
			"Senhas são armazenadas apenas como hash com algoritmo de custo adaptativo",
			"Mensagens de erro nunca revelam se o e-mail existe na base",
		},
		Acceptance: []string{
			"**Given** um usuário cadastrado com credenciais válidas **When** envia `POST /api/v1/auth/login` **Then** recebe HTTP 200 com `{\"token\": \"<jwt>\", \"expires_in\": 3600}`.",
			"**Given** um usuário com senha incorreta **When** envia `POST /api/v1/auth/login` **Then** recebe HTTP 401 sem indicar qual campo falhou.",
		},
	}
}

func starterADR() *model.ADR {
	return &model.ADR{
		ID: "ADR-001", Title: "Autenticação baseada em JWT stateless", Status: "Aceito",
		Context: "O sistema precisa autenticar clientes web e mobile sem manter sessão em memória no servidor, " +
			"permitindo escalar horizontalmente a Core API sem sticky sessions.",
		Decision: "Adotar JWT assinado (HS256 na fase inicial, migrando para RS256 quando houver múltiplos emissores), " +
			"com validade curta de 1 hora e refresh token rotativo persistido no PostgreSQL.",
		Consequences: "Positivo: escala horizontal trivial e validação local barata. " +
			"Negativo: revogação imediata exige lista de bloqueio em Redis, já prevista na arquitetura.",
	}
}

const useCaseTemplate = `# CDU000 — Nome do Caso de Uso

- **Atores:** -
- **Componentes:** -
- **Complexidade:** medium
- **Horas Estimadas:** 16
- **Prioridade:** Média
- **Status:** pending

## Pré-condições

- _Nenhuma_

## Fluxo Principal

1. _A definir._

## Fluxos Alternativos

- _Nenhum_

## Exceções

- _Nenhuma_

## Regras de Negócio

- _Nenhuma_

## Critérios de Aceite

- **Given** _estado inicial_ **When** _ação do ator_ **Then** _resultado observável_.
`
