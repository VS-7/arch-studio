# ==============================================================================
# ArchCode Studio - Dockerfile Multi-Stage (mesmo padrão do Harmoni, testado no Coolify)
# ==============================================================================

# ------------------------------------------------------------------------------
# Stage 1: Build do Frontend (React + TypeScript + Vite)
# ------------------------------------------------------------------------------
FROM node:22-alpine AS frontend-builder
WORKDIR /app/web

COPY web/package*.json ./
RUN npm ci --prefer-offline --no-audit

# O Vite emite o bundle em ../internal/webui/dist (embutido via go:embed)
COPY web/ ./
RUN mkdir -p /app/internal/webui/dist && npm run build

# ------------------------------------------------------------------------------
# Stage 2: Build do Backend (Go)
# ------------------------------------------------------------------------------
FROM golang:1.25-alpine AS backend-builder
WORKDIR /app

RUN apk add --no-cache git ca-certificates tzdata

COPY go.mod go.sum ./
RUN go mod download

COPY cmd/ ./cmd/
COPY internal/ ./internal/

# Assets gerados no Stage 1 para o local embutido pelo go:embed
COPY --from=frontend-builder /app/internal/webui/dist/ ./internal/webui/dist/

ARG VERSION=1.0.0
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath \
    -ldflags "-s -w -X main.version=${VERSION}" \
    -o /bin/archcode-studio ./cmd/archcode-studio

# ------------------------------------------------------------------------------
# Stage 3: Runtime
# ------------------------------------------------------------------------------
FROM alpine:3.21 AS runtime

RUN apk add --no-cache ca-certificates tzdata curl \
    && rm -rf /var/cache/apk/*

WORKDIR /workspace

COPY --from=backend-builder /bin/archcode-studio /usr/local/bin/archcode-studio
COPY docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
RUN chmod +x /usr/local/bin/docker-entrypoint.sh

# /workspace guarda o projeto (.arch/, docs/, api/): monte como volume persistente.
ENV PORT=8765 \
    ARCHCODE_DIR=/workspace

EXPOSE 8765

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD curl -f http://localhost:${PORT}/api/health || exit 1

ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]
CMD ["serve"]
