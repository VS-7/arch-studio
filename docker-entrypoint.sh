#!/bin/sh
# Inicializa o projeto no volume na primeira execução e sobe o servidor.
set -e

WORKSPACE="${ARCHCODE_DIR:-/workspace}"

if [ "$1" = "serve" ] && [ ! -f "$WORKSPACE/.arch/manifest.yaml" ]; then
    echo "Nenhum projeto em $WORKSPACE — criando \"${ARCHCODE_PROJECT_NAME:-ArchCode Project}\"…"
    archcode-studio init --dir "$WORKSPACE" --name "${ARCHCODE_PROJECT_NAME:-ArchCode Project}"
fi

if [ "$1" = "serve" ]; then
    shift
    exec archcode-studio serve --dir "$WORKSPACE" --host 0.0.0.0 --port "${PORT:-8765}" --open=false "$@"
fi

exec archcode-studio "$@"
