#!/usr/bin/env bash
# MaskChain quickstart — one command to a working gateway.
#
#   ./quickstart.sh            # local Ollama (zero external API keys)
#   ./quickstart.sh openai     # OpenAI (needs OPENAI_KEY in .env)
#
# Generates .env with fresh encryption keys on first run.

set -euo pipefail

cd "$(dirname "$0")"

MODE="${1:-ollama}"
GATEWAY_URL="http://localhost:8080"
ADMIN_URL="http://localhost:9090"

case "$MODE" in
  ollama)
    COMPOSE="docker-compose.ollama.yml"
    MODEL="${OLLAMA_MODEL:-llama3.2}"
    ;;
  openai)
    COMPOSE="docker-compose.openai.yml"
    MODEL="gpt-4o-mini"
    ;;
  *)
    echo "usage: $0 [ollama|openai]" >&2
    exit 1
    ;;
esac

if [ ! -f .env ]; then
  echo "==> Generating .env with fresh encryption keys"
  {
    printf 'MASKCHAIN_KEYS_KEY=%s\n' "$(openssl rand -base64 32)"
    printf 'MASKCHAIN_CONVERSATION_KEY=%s\n' "$(openssl rand -base64 32)"
    if [ -n "${OPENAI_KEY:-}" ]; then
      printf 'OPENAI_KEY=%s\n' "$OPENAI_KEY"
    fi
  } > .env
fi

if [ "$MODE" = "openai" ] && ! grep -qE '^OPENAI_KEY=.+' .env; then
  if [ -n "${OPENAI_KEY:-}" ]; then
    printf 'OPENAI_KEY=%s\n' "$OPENAI_KEY" >> .env
  else
    echo "ERROR: set OPENAI_KEY in $(pwd)/.env (or pass OPENAI_KEY=... ), then re-run." >&2
    exit 1
  fi
fi

echo "==> Starting MaskChain ($MODE mode). First build can take a few minutes."
docker compose -f "$COMPOSE" up -d --build

echo "==> Waiting for gateway at ${GATEWAY_URL}/health ..."
for _ in $(seq 1 60); do
  if curl -sf "${GATEWAY_URL}/health" >/dev/null 2>&1; then
    echo "==> Gateway is up."
    break
  fi
  sleep 2
done

if ! curl -sf "${GATEWAY_URL}/health" >/dev/null 2>&1; then
  echo "Gateway did not become healthy. Recent logs:" >&2
  docker compose -f "$COMPOSE" logs --tail=40 maskchain >&2 || true
  exit 1
fi

cat <<EOF

MaskChain is running.

  Gateway  ${GATEWAY_URL}          (Authorization: Bearer sk-test-default)
  Admin    ${ADMIN_URL}            (admin / test)
  Swagger  ${ADMIN_URL}/api/v1/docs

Try a masked request:

  curl -s ${GATEWAY_URL}/api/v1/chat/completions \\
    -H "Authorization: Bearer sk-test-default" \\
    -H "Content-Type: application/json" \\
    -d '{"model":"${MODEL}","messages":[{"role":"user","content":"My email is alice@example.com, call me at +1-555-0100"}]}'

Next: read README.md, then examples/cookbook/ for grouped recipes.
Stop the stack: docker compose -f ${COMPOSE} down
EOF
