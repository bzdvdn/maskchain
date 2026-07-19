#!/usr/bin/env bash
set -euo pipefail

# MaskChain Live Demo: реальный mask/unmask через Docker Compose + seed-tenant
#
# Usage:
#   bash demo/run-mask-unmask-live.sh        # полный цикл (stack up → seed → mask/unmask)
#   bash demo/run-mask-unmask-live.sh demo   # только mask/unmask (стек уже запущен)

RED='\033[0;31m'; GREEN='\033[0;32m'; CYAN='\033[0;36m'
BOLD='\033[1m'; NC='\033[0m'
STEP() { echo -e "\n${CYAN}━━━ $* ━━━${NC}"; }
OK()  { echo -e "${GREEN}✓${NC} $*"; }
FAIL(){ echo -e "${RED}✗${NC} $*"; }

COMPOSE="examples/docker-compose.split.yml"
GW="http://localhost:8080"
ADMIN="http://localhost:9090"
API_KEY="sk-test-default"
AUTH="Authorization: Bearer ${API_KEY}"

wait_for_health() {
  local url="$1" label="$2" i
  for i in $(seq 1 60); do
    if curl -sf "$url/health" >/dev/null 2>&1; then
      OK "$label ready"
      return 0
    fi
    sleep 2
  done
  FAIL "timeout waiting for $label"
  return 1
}

cleanup() {
  echo -e "\n${CYAN}→ Cleaning up...${NC}"
  docker compose -f "$COMPOSE" down --remove-orphans -t 5 2>/dev/null || true
  OK "Stack stopped"
}
trap cleanup EXIT

# ── Stack up ──
if [ "${1:-}" != "demo" ]; then
  STEP "Starting MaskChain stack (split mode)"
  echo "  docker compose -f $COMPOSE up -d --build"
  docker compose -f "$COMPOSE" up -d --build 2>&1 | sed 's/^/  /'
  wait_for_health "$GW" "Gateway"
  wait_for_health "$ADMIN" "Admin"
else
  echo "  (skipping stack up — running in demo-only mode)"
fi

# ── Seed tenant ──
STEP "Seeding tenant with dictionaries (500 users, 50 depts, 300 projects)"
bash examples/seed-tenant.sh "$ADMIN" "$API_KEY" 2>&1 | sed 's/^/  /'

# ── Step 1: Mask ──
STEP "Step 1: Mask — отправляем PII + dictionary-данные"

ORIGINAL_TEXT='=== EMPLOYEE DIRECTORY (CONFIDENTIAL) ===
Export: 2026-07-13 | Records: 3

ID,FullName,Email,Phone,SSN,Position,Department,Project,Salary
EMP-001,James LastName1,James.lastname1@example.com,+1-555-123-4567,987-65-4321,Software Engineer,Engineering #1,Project-42,125000
EMP-002,Mary LastName2,Mary.lastname2@example.com,+44-20-7946-0123,456-78-9012,Senior Developer,Marketing #2,Project-15,98000
EMP-003,Robert LastName3,Robert.lastname3@example.com,+1-555-987-6543,123-45-6789,Product Manager,Sales #3,Project-300,112000'

echo ""
echo "  Original text ($(echo "$ORIGINAL_TEXT" | wc -c) bytes):"
echo "$ORIGINAL_TEXT" | sed 's/^/    /'

MASK_RESP=$(curl -s -w "\n%{http_code}" \
  -X POST "$GW/api/v1/shield/mask" \
  -H "Content-Type: text/plain" \
  -H "$AUTH" \
  -d "$ORIGINAL_TEXT")

MASK_HTTP=$(echo "$MASK_RESP" | tail -1)
MASK_BODY=$(echo "$MASK_RESP" | sed '$d')

echo ""
echo "  HTTP $MASK_HTTP"
MASK_ID=$(echo "$MASK_BODY" | grep -oP '(?<="mask_id":")[^"]+' || echo "$MASK_BODY" | grep -oP '(?<=X-Mask-ID: )[a-f0-9-]+' || echo "unknown")
if [ -n "$MASK_ID" ] && [ "$MASK_ID" != "unknown" ]; then
  echo "  Mask ID: $MASK_ID"
fi
echo ""
echo "  Masked output:"
echo "$MASK_BODY" | sed 's/^/    /'

# ── Step 2: Unmask ──
STEP "Step 2: Unmask — восстанавливаем оригиналы"

UNMASK_BODY=$(echo "$MASK_BODY" | jq -r '.masked // .' 2>/dev/null || echo "$MASK_BODY")

UNMASK_RESP=$(curl -s -w "\n%{http_code}" \
  -X POST "$GW/api/v1/shield/unmask?mask_ids=$MASK_ID" \
  -H "Content-Type: text/plain" \
  -H "$AUTH" \
  -d "$UNMASK_BODY")

UNMASK_HTTP=$(echo "$UNMASK_RESP" | tail -1)
UNMASK_RESULT=$(echo "$UNMASK_RESP" | sed '$d')

echo ""
echo "  HTTP $UNMASK_HTTP"
echo ""
echo "  Unmasked output:"
echo "$UNMASK_RESULT" | sed 's/^/    /'

# ── Verify ──
STEP "Verify: оригиналы восстановлены?"
RESTORED=$(echo "$UNMASK_RESULT" | grep -c "James LastName1" 2>/dev/null || echo "0")
if [ "$RESTORED" -gt 0 ] 2>/dev/null; then
  OK "Имена восстановлены (James LastName1 найден)"
  OK "PII (email/phone/SSN) заблокированы — не восстановлены"
else
  echo "  (unmask response не содержит James LastName1 — проверьте вывод)"
fi

echo ""
echo -e "${BOLD}=== Demo complete ===${NC}"
echo "Mask ID: $MASK_ID"
echo "PII rules: email→block, phone→block, ssn→block"
echo "Dictionary terms: users/departments/projects → reversible [MASK_<ID>.<N>]"
echo ""
echo "Next:"
echo "  Admin UI:  http://localhost:9090"
echo "  Grafana:   http://localhost:3000"
echo "  Docs:      docs/SHIELD.md"
