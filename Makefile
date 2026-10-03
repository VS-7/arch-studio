# ArchCode Studio — automação de build.
#
# `make build` produz um binário único, sem dependências externas, com o
# frontend React compilado embutido via embed.FS. `make desktop` produz o app
# desktop (Wails v3, módulo em desktop/) com o mesmo frontend.

BINARY      := archcode-studio
VERSION     ?= 1.3.1
LDFLAGS     := -s -w -X main.version=$(VERSION)
GO          ?= go
NPM         ?= npm
PLATFORMS   := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64

.PHONY: help
help: ## Mostra esta ajuda
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

.PHONY: deps
deps: ## Instala dependências de Go e do frontend
	$(GO) mod download
	$(NPM) --prefix web ci || $(NPM) --prefix web install

.PHONY: web
web: ## Compila o frontend para internal/webui/dist
	$(NPM) --prefix web run build

.PHONY: build
build: web ## Compila o binário único com o frontend embutido
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/archcode-studio
	@echo "✓ $(BINARY) pronto ($$(du -h $(BINARY) | cut -f1))"

.PHONY: build-server
build-server: ## Compila apenas o backend (usa o dist existente)
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/archcode-studio

.PHONY: dev
dev: ## Sobe backend (8765) e Vite com hot reload (5173)
	@echo "Backend em :8765 · Frontend em :5173 (proxy para o backend)"
	@($(GO) run ./cmd/archcode-studio serve --open=false &) && $(NPM) --prefix web run dev

.PHONY: test
test: ## Roda os testes Go
	$(GO) test ./...

.PHONY: test-race
test-race: ## Roda os testes com o detector de corrida
	$(GO) test -race ./...

.PHONY: cover
cover: ## Gera o relatório de cobertura
	$(GO) test -coverprofile=coverage.out ./...
	$(GO) tool cover -func=coverage.out | tail -1

.PHONY: typecheck
typecheck: ## Verifica os tipos do frontend
	$(NPM) --prefix web run typecheck

.PHONY: lint
lint: ## Formatação e análise estática
	gofmt -l . | (! grep .) || (echo "arquivos mal formatados acima"; exit 1)
	$(GO) vet ./...

.PHONY: check
check: lint test typecheck desktop-test ## Executa tudo que a CI executa (menos o build nativo do desktop)

.PHONY: release
release: web ## Compila binários para todas as plataformas em dist/
	@mkdir -p dist
	@for platform in $(PLATFORMS); do \
		os=$${platform%/*}; arch=$${platform#*/}; \
		ext=""; [ "$$os" = "windows" ] && ext=".exe"; \
		echo "  → $$os/$$arch"; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch $(GO) build -trimpath \
			-ldflags "$(LDFLAGS)" -o dist/$(BINARY)-$$os-$$arch$$ext ./cmd/archcode-studio; \
	done
	@echo "✓ binários em dist/"

# ---------------------------------------------------------------------------
# App desktop (Wails v3) — módulo Go próprio em desktop/, que exige CGO no Linux
# e no macOS. Linux: libgtk-4-dev e libwebkitgtk-6.0-dev (ou, com
# DESKTOP_TAGS="production gtk3", libgtk-3-dev e libwebkit2gtk-4.1-dev).
# macOS: Xcode Command Line Tools (xcode-select --install).
# ---------------------------------------------------------------------------

DESKTOP_BIN  := archcode-desktop
DESKTOP_TAGS ?= production

.PHONY: desktop
desktop: web ## Compila o app desktop para o sistema atual em dist/
	@mkdir -p dist
	cd desktop && CGO_ENABLED=1 $(GO) build -trimpath -tags "$(DESKTOP_TAGS)" -ldflags "$(LDFLAGS)" -o ../dist/$(DESKTOP_BIN) .
	@echo "✓ dist/$(DESKTOP_BIN) pronto"

.PHONY: desktop-windows
desktop-windows: web ## Compila o app desktop para Windows (sem CGO, funciona de qualquer sistema)
	@mkdir -p dist
	cd desktop && CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GO) build -trimpath -tags "$(DESKTOP_TAGS)" \
		-ldflags "$(LDFLAGS) -H windowsgui" -o ../dist/$(DESKTOP_BIN)-windows-amd64.exe .
	@echo "✓ dist/$(DESKTOP_BIN)-windows-amd64.exe pronto"

# macOS: o app precisa ser um bundle .app (Info.plist + ícone) — um executável
# solto baixado pelo navegador perde a permissão de execução e abre no Terminal.
# O alvo de implantação acompanha o Go (macOS 12) e evita os avisos do linker.
MACOS_APP := dist/ArchCode Studio.app
MACOS_ZIP := archcode-desktop-$(VERSION)-macos-universal.zip
MACOS_ENV := CGO_ENABLED=1 MACOSX_DEPLOYMENT_TARGET=12.0 \
	CGO_CFLAGS=-mmacosx-version-min=12.0 CGO_LDFLAGS=-mmacosx-version-min=12.0

.PHONY: desktop-macos
desktop-macos: web ## Compila o ArchCode Studio.app universal (Apple Silicon + Intel) em dist/ — só no macOS
	@mkdir -p dist
	@for arch in arm64 amd64; do \
		echo "  → darwin/$$arch"; \
		(cd desktop && $(MACOS_ENV) GOARCH=$$arch $(GO) build -trimpath -tags "$(DESKTOP_TAGS)" \
			-ldflags "$(LDFLAGS)" -o ../dist/$(DESKTOP_BIN)-darwin-$$arch .) || exit 1; \
	done
	rm -rf "$(MACOS_APP)" && mkdir -p "$(MACOS_APP)/Contents/MacOS" "$(MACOS_APP)/Contents/Resources"
	lipo -create -output "$(MACOS_APP)/Contents/MacOS/$(DESKTOP_BIN)" \
		dist/$(DESKTOP_BIN)-darwin-arm64 dist/$(DESKTOP_BIN)-darwin-amd64
	rm dist/$(DESKTOP_BIN)-darwin-arm64 dist/$(DESKTOP_BIN)-darwin-amd64
	sed 's/__VERSION__/$(VERSION)/g' desktop/build/darwin/Info.plist > "$(MACOS_APP)/Contents/Info.plist"
	@# Ícone .icns gerado do appicon.png (512 px), nos tamanhos que o Finder e o Dock usam.
	@set -e; iconset=$$(mktemp -d)/icons.iconset; mkdir -p $$iconset; \
		for s in 16 32 128 256; do \
			sips -z $$s $$s desktop/appicon.png --out $$iconset/icon_$${s}x$${s}.png >/dev/null; \
			sips -z $$((s*2)) $$((s*2)) desktop/appicon.png --out $$iconset/icon_$${s}x$${s}@2x.png >/dev/null; \
		done; \
		cp desktop/appicon.png $$iconset/icon_512x512.png; \
		iconutil -c icns $$iconset -o "$(MACOS_APP)/Contents/Resources/icons.icns"; \
		rm -rf $$(dirname $$iconset)
	@# Assinatura ad-hoc: sem ela o macOS em Apple Silicon recusa o app ("danificado").
	codesign --force --sign - "$(MACOS_APP)"
	@# Sem atributos estendidos: eles virariam arquivos ._* que quebram a assinatura
	@# quando o .zip é extraído por outra ferramenta que não o Utilitário de Arquivo.
	cd dist && rm -f $(MACOS_ZIP) && \
		ditto -c -k --norsrc --noextattr --noacl --keepParent "ArchCode Studio.app" $(MACOS_ZIP)
	@echo "✓ $(MACOS_APP) e dist/$(MACOS_ZIP) prontos"

NFPM   ?= $(GO) run github.com/goreleaser/nfpm/v2/cmd/nfpm@v2.47.0
GOARCH ?= $(shell $(GO) env GOARCH)

.PHONY: desktop-package
desktop-package: desktop ## Empacota o app desktop Linux em dist/ (.deb, .rpm e .tar.gz)
	@for fmt in deb rpm; do \
		VERSION=$(VERSION) GOARCH=$(GOARCH) $(NFPM) pkg --config desktop/build/linux/nfpm.yaml --packager $$fmt --target dist/ || exit 1; \
	done
	@dir=archcode-desktop-$(VERSION)-linux-$(GOARCH); rm -rf dist/$$dir && mkdir -p dist/$$dir && \
		cp dist/$(DESKTOP_BIN) desktop/build/linux/io.archcode.studio.desktop dist/$$dir/ && \
		cp desktop/appicon.png dist/$$dir/archcode-studio.png && \
		tar -czf dist/$$dir.tar.gz -C dist $$dir && rm -rf dist/$$dir
	@ls -1 dist/ | sed 's/^/  ✓ dist\//'

.PHONY: desktop-dev
desktop-dev: web ## Roda o app desktop em modo de desenvolvimento (DevTools habilitado)
	cd desktop && CGO_ENABLED=1 $(GO) run .

.PHONY: desktop-test
desktop-test: ## Testes do app desktop que não dependem da interface gráfica
	cd desktop && $(GO) vet ./internal/... && $(GO) test ./internal/...

.PHONY: docker
docker: ## Constrói a imagem Docker
	docker build -t ghcr.io/archcode/studio:$(VERSION) -t ghcr.io/archcode/studio:latest .

.PHONY: clean
clean: ## Remove artefatos de build
	rm -rf $(BINARY) dist coverage.out internal/webui/dist/assets internal/webui/dist/index.html
