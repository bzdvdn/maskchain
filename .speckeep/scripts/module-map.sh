#!/bin/sh

set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"

ROOT_DIR="$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)"

# Module map: pass through the subcommand and append the project root so the
# command works from any working directory.
if [ $# -eq 0 ]; then
  exec "$SCRIPT_DIR/run-speckeep.sh" module-map show "$ROOT_DIR"
fi

exec "$SCRIPT_DIR/run-speckeep.sh" module-map "$@" "$ROOT_DIR"
