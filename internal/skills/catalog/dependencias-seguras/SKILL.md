---
name: dependencias-seguras
description: Garante que toda dependência está fixada com lockfile versionado, sem vulnerabilidade conhecida, mantida, com licença compatível e justificada no PR. Use antes do PR de qualquer tarefa que adicione, remova ou atualize pacotes.
category: seguranca
version: 1.0.0
trigger: antes_do_pr
checks:
  - id: lockfile-versionado
    text: Manifesto e lockfile (go.mod e go.sum, package.json e package-lock.json, poetry.lock…) estão commitados e coerentes entre si.
    required: true
  - id: go-mod-arrumado
    text: Em módulos Go, go.mod e go.sum não têm diferenças pendentes de go mod tidy.
    command: go mod tidy -diff
    required: false
  - id: govulncheck-limpo
    text: Em projetos Go, o govulncheck não reporta vulnerabilidade alcançável.
    command: govulncheck ./...
    required: true
  - id: npm-audit-limpo
    text: Em projetos Node, o npm audit não reporta vulnerabilidade high ou critical.
    command: npm audit --audit-level=high
    required: true
  - id: dependencia-justificada
    text: Toda dependência nova tem uma linha no corpo do PR com finalidade, motivo de a stdlib não bastar, licença e último release.
    required: true
  - id: licenca-compativel
    text: A licença de cada dependência nova é compatível com a licença do projeto.
    required: true
  - id: pacote-mantido
    text: Nenhuma dependência nova está arquivada, depreciada ou sem release nos últimos 2 anos.
    required: false
---

# Dependências seguras

## Quando usar

Antes do PR de toda tarefa que mexa em `go.mod`, `package.json`, `requirements.txt`, `pyproject.toml`, `pom.xml` ou equivalentes — inclusive atualização de versão. Cada pacote novo é código de terceiros rodando com os privilégios do sistema: trate-o como uma pequena decisão de arquitetura.

## Regras

1. **Prefira o que já existe.** Antes de adicionar, procure na stdlib (`net/http`, `encoding/json`, `slices`, `log/slog`; `fetch`, `URL`, `Intl`, `crypto.randomUUID`) e nas dependências já presentes. Não adicione pacote para uma função de 10 linhas.
2. **Fixe versões e versione o lockfile.** Commite `go.sum`, `package-lock.json`/`pnpm-lock.yaml`/`yarn.lock`, `poetry.lock`/`uv.lock`. Use o gerenciador do projeto para alterar dependências; nunca edite lockfile à mão. Em CI, `npm ci`.
3. **Audite.** Go: `govulncheck ./...`. Node: `npm audit --audit-level=high`. Python: `pip-audit`. Vulnerabilidade alta ou crítica alcançável bloqueia o PR: atualize ou troque o pacote; se a correção não couber na tarefa, registre `create_backlog_item` com `type: security` e diga isso no PR.
4. **Confira saúde e procedência.** Repositório ativo, release nos últimos 2 anos, mantenedores identificáveis, nome exato (cuidado com typosquatting: `lodash` ≠ `lodahs`). Desconfie de scripts `postinstall`.
5. **Confira a licença.** MIT, BSD, Apache-2.0 e ISC: ok. GPL, AGPL, SSPL ou sem licença em projeto proprietário: pare e peça decisão humana; se aprovada, registre com `upsert_adr`.
6. **Justifique no PR**, uma linha por dependência nova:
   `- github.com/golang-jwt/jwt/v5 v5.2.1 — validação de JWT; stdlib não tem; MIT; release há 3 meses.`
7. **Upgrade grande é tarefa própria.** Troca de versão major não se mistura com feature.
8. **Registre armadilhas.** Versão que quebrou algo vira `remember` com `type: armadilha`, citando pacote e versão.

## Como verificar

- `lockfile-versionado`: quando o manifesto muda no diff, o lockfile também muda; `git status --porcelain` fica limpo após instalar.
- `go-mod-arrumado`: `go mod tidy -diff` não imprime nada (requer Go 1.23+; `na` sem Go).
- `govulncheck-limpo`: `govulncheck ./...` termina com código 0 (`na` sem Go).
- `npm-audit-limpo`: `npm audit --audit-level=high` termina com código 0 (`na` sem Node).
- `dependencia-justificada`: o corpo do PR tem a linha da regra 6 para cada pacote novo.
- `licenca-compativel`: a licença de cada pacote novo está citada na justificativa e é permitida pela regra 5.
- `pacote-mantido`: a data do último release está citada na justificativa.

## Exemplos

- ❌ `npm install moment` para formatar data → ✅ `new Intl.DateTimeFormat('pt-BR').format(d)`.
- ❌ `go get github.com/pkg/errors` → ✅ `fmt.Errorf("...: %w", err)` com `errors.Is`/`errors.As`.
- ❌ `"axios": "^1"` sem lockfile commitado → ✅ versão resolvida registrada no `package-lock.json` do commit.
