# Build do frontend -----------------------------------------------------------
FROM node:20-alpine AS web
WORKDIR /src
COPY web/package.json web/package-lock.json ./web/
RUN npm --prefix web ci
COPY web ./web
RUN mkdir -p internal/webui/dist && npm --prefix web run build

# Build do binário ------------------------------------------------------------
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/internal/webui/dist ./internal/webui/dist
ARG VERSION=1.0.0
RUN CGO_ENABLED=0 go build -trimpath \
    -ldflags "-s -w -X main.version=${VERSION}" \
    -o /out/archcode-studio ./cmd/archcode-studio

# Imagem final ----------------------------------------------------------------
FROM alpine:3.20
RUN adduser -D -u 10001 archcode \
    && mkdir -p /workspace \
    && chown archcode:archcode /workspace
COPY --from=build /out/archcode-studio /usr/local/bin/archcode-studio
COPY --chmod=755 docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
WORKDIR /workspace
USER archcode

# O disco continua sendo a fonte da verdade: monte /workspace como volume
# persistente (no Coolify: Storages → Volume Mount em /workspace).
#   docker run -p 8765:8765 -v "$PWD:/workspace" ghcr.io/archcode/studio
VOLUME ["/workspace"]
ENV PORT=8765
EXPOSE 8765

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- "http://127.0.0.1:${PORT}/api/health" >/dev/null || exit 1

ENTRYPOINT ["docker-entrypoint.sh"]
CMD ["serve"]
