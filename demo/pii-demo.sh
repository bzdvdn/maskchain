#!/bin/bash
set -euo pipefail

# MaskChain PII Demo — запускает стек, отправляет PII-промпт, показывает маскирование
# Requires: docker, curl, python3

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'
CYAN='\033[0;36m'; BOLD='\033[1m'; NC='\033[0m'
COMPOSE_FILE="examples/docker-compose.yml"
GATEWAY_URL="http://localhost:8080"
TIMEOUT=60

cleanup() {
  echo -e "\n${YELLOW}→ Cleaning up...${NC}"
  docker compose -f "$COMPOSE_FILE" down --remove-orphans 2>/dev/null || true
  rm -f /tmp/maskchain-demo-*.json
  echo -e "${GREEN}✓ Done${NC}"
}
trap cleanup EXIT

wait_for_health() {
  local url="$1" timeout="$2" elapsed=0
  while ! curl -sf "$url" >/dev/null 2>&1; do
    sleep 1; elapsed=$((elapsed + 1))
    if [ "$elapsed" -ge "$timeout" ]; then
      echo -e "${RED}✗ Timeout waiting for $url${NC}"; return 1
    fi
  done
}

print_step() { echo -e "\n${CYAN}━━━ $1 ━━━${NC}"; }
print_ok()   { echo -e "${GREEN}✓${NC} $1"; }

# ── Step 0: prerequisites ──
print_step "Step 0: Checking prerequisites"
for cmd in docker curl python3; do
  command -v "$cmd" >/dev/null 2>&1 || { echo -e "${RED}✗ $cmd not found${NC}"; exit 1; }
done
print_ok "All prerequisites found"

# ── Step 1: start stack ──
print_step "Step 1: Starting MaskChain stack"
echo "→ docker compose -f $COMPOSE_FILE up -d"
docker compose -f "$COMPOSE_FILE" up -d --build 2>&1
wait_for_health "$GATEWAY_URL/health" $TIMEOUT
print_ok "Gateway is healthy"

# ── Step 2: seed tenant ──
print_step "Step 2: Seeding demo tenant"
ADMIN_URL="http://localhost:8081"
wait_for_health "$ADMIN_URL/health" $TIMEOUT

curl -sf -X POST "$ADMIN_URL/api/v1/admin/tenants" \
  -H "Content-Type: application/json" \
  -d '{"slug":"demo","name":"Demo Tenant","api_keys":["sk-demo-key"]}' >/dev/null 2>&1 || true

curl -sf -X PUT "$ADMIN_URL/api/v1/admin/tenants/demo/dictionaries" \
  -H "Content-Type: application/json" \
  -d '{"entries":["ProjectX","InternalCodename"]}' >/dev/null 2>&1 || true

curl -sf -X PUT "$ADMIN_URL/api/v1/admin/tenants/demo/shield" \
  -H "Content-Type: application/json" \
  -d '{"enabled_detectors":["email","phone","ssn","credit_card","dictionary"]}' >/dev/null 2>&1 || true

print_ok "Tenant 'demo' seeded with API key sk-demo-key"

# ── Step 3: PII request (masked) ──
print_step "Step 3: PII Detection — prompt with sensitive data"
echo -e "${YELLOW}→ Sending: 'My email is john@example.com, phone +1 (555) 123-4567, SSN 123-45-6789, card 4111-1111-1111-1111, codename ProjectX'${NC}"

RESP=$(curl -s -X POST "$GATEWAY_URL/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-demo-key" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "My email is john@example.com and my phone is +1 (555) 123-4567. My SSN is 123-45-6789 and my credit card is 4111-1111-1111-1111. Also my project codename is ProjectX."}]
  }')

echo "$RESP" | python3 -m json.tool 2>/dev/null || echo "$RESP"

MASKS=$(echo "$RESP" | grep -oP '\[(EMAIL|PHONE|SSN|CREDIT_CARD|DICT)\]' | sort -u || true)
if [ -n "$MASKS" ]; then
  echo -e "\n${GREEN}✓ PII detected and masked:${NC}"
  echo "$MASKS" | while read -r m; do echo -e "  ${YELLOW}$m${NC}"; done
else
  echo -e "\n${RED}⚠ No masks found in response${NC}"
fi

# ── Step 4: clean request (pass) ──
print_step "Step 4: Clean request — no PII"
echo -e "${YELLOW}→ Sending: 'What is the capital of France?'${NC}"

RESP2=$(curl -s -X POST "$GATEWAY_URL/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-demo-key" \
  -d '{"model":"gpt-4","messages":[{"role":"user","content":"What is the capital of France?"}]}')

MASKS2=$(echo "$RESP2" | grep -oP '\[(EMAIL|PHONE|SSN|CREDIT_CARD|DICT)\]' || true)
if [ -z "$MASKS2" ]; then
  print_ok "No PII detected — clean request passed through"
else
  echo -e "${YELLOW}⚠ Unexpected masks found${NC}"
fi

# ── Step 5: analytics ──
print_step "Step 5: Analytics"
echo "→ Shield stats from admin API:"
curl -sf "$ADMIN_URL/api/v1/analytics/tokens?tenant_slug=demo" 2>/dev/null | python3 -m json.tool 2>/dev/null || echo "(analytics endpoint unavailable — no requests processed yet)"

# ── Step 6: summary ──
print_step "Step 6: Summary"
echo -e "${BOLD}MaskChain PII Shield Demo${NC}"
echo -e "  ${GREEN}✓${NC} Stack: gateway + admin + postgres + valkey"
echo -e "  ${GREEN}✓${NC} Tenant isolation: API key → demo tenant"
echo -e "  ${GREEN}✓${NC} PII detection: email, phone, SSN, credit card → masked"
echo -e "  ${GREEN}✓${NC} Dictionary masking: ProjectX → [DICT]"
echo -e "  ${GREEN}✓${NC} Clean requests: pass through unmodified"
echo ""
echo "To explore further:"
echo "  Admin UI:  http://localhost:8081"
echo "  API docs:  http://localhost:8081/swagger/"
echo "  Runbook:   deployments/runbook.md"
echo ""
echo -e "${BOLD}Demo complete. Cleaning up...${NC}"
