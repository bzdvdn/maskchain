#!/usr/bin/env bash
# @sk-test 80-tenant-isolation#T4.4: Manual validation script for multi-tenant isolation
#
# First Validation Path (from plan.md):
#   alpha: Authorization:Bearer sk-abc123 → profiles for tenant alpha
#   beta:  X-Mask-Authorization mk-beta-key → profiles for tenant beta
#   gamma: X-Custom-Token custom-gamma-token → profiles for tenant gamma
#   no key → 401
#   wrong key → 401
#
# Prerequisites:
#   1. Gateway running on localhost:8080 (or set MASKCHAIN_URL)
#   2. Config with tenants: alpha, beta, gamma (see plan.md for full config)
#   3. curl and jq installed
set -euo pipefail

BASE="${MASKCHAIN_URL:-http://localhost:8080}"
PASS=0
FAIL=0

pass() {
  echo "  ✅ $1"
  ((PASS++))
}

fail() {
  echo "  ❌ $1"
  ((FAIL++))
}

check_status() {
  local desc="$1" expected="$2" actual="$3"
  if [[ "$actual" == "$expected" ]]; then
    pass "$desc (HTTP $actual)"
  else
    fail "$desc — expected HTTP $expected, got $actual"
  fi
}

echo "================================================"
echo "  Tenant Isolation — Manual Validation"
echo "  Base URL: $BASE"
echo "================================================"
echo ""

# --- 1. Valid Bearer (alpha) ---
echo "--- 1. Valid Bearer (alpha) ---"
resp=$(curl -s -w "%{http_code}" -o /tmp/alpha.json -H "Authorization: Bearer sk-abc123" "$BASE/api/v1/profiles")
status="${resp: -3}"
check_status "alpha Bearer → 200" "200" "$status"
jq . /tmp/alpha.json 2>/dev/null || echo "  (raw: $(cat /tmp/alpha.json))"
echo ""

# --- 2. Default X-Mask-Authorization (beta) ---
echo "--- 2. Default X-Mask-Authorization (beta) ---"
resp=$(curl -s -w "%{http_code}" -o /tmp/beta.json -H "X-Mask-Authorization: mk-beta-key" "$BASE/api/v1/profiles")
status="${resp: -3}"
check_status "beta X-Mask-Authorization → 200" "200" "$status"
jq . /tmp/beta.json 2>/dev/null || echo "  (raw: $(cat /tmp/beta.json))"
echo ""

# --- 3. Custom header (gamma) ---
echo "--- 3. Custom header (gamma) ---"
resp=$(curl -s -w "%{http_code}" -o /tmp/gamma.json -H "X-Custom-Token: custom-gamma-token" "$BASE/api/v1/profiles")
status="${resp: -3}"
check_status "gamma custom header → 200" "200" "$status"
jq . /tmp/gamma.json 2>/dev/null || echo "  (raw: $(cat /tmp/gamma.json))"
echo ""

# --- 4. No auth header → 401 ---
echo "--- 4. No auth header → 401 ---"
resp=$(curl -s -w "%{http_code}" -o /tmp/noauth.json "$BASE/api/v1/profiles")
status="${resp: -3}"
check_status "no auth header → 401" "401" "$status"
jq . /tmp/noauth.json 2>/dev/null || echo "  (raw: $(cat /tmp/noauth.json))"
echo ""

# --- 5. Invalid key → 401 ---
echo "--- 5. Invalid key → 401 ---"
resp=$(curl -s -w "%{http_code}" -o /tmp/badkey.json -H "Authorization: Bearer wrong-key" "$BASE/api/v1/profiles")
status="${resp: -3}"
check_status "invalid key → 401" "401" "$status"
jq . /tmp/badkey.json 2>/dev/null || echo "  (raw: $(cat /tmp/badkey.json))"
echo ""

# --- 6. Key theft: alpha key in wrong header → 401 ---
echo "--- 6. Key theft: alpha key in X-Mask-Authorization (wrong header) → 401 ---"
resp=$(curl -s -w "%{http_code}" -o /tmp/theft.json -H "X-Mask-Authorization: sk-abc123" "$BASE/api/v1/profiles")
status="${resp: -3}"
check_status "key theft → 401" "401" "$status"
jq . /tmp/theft.json 2>/dev/null || echo "  (raw: $(cat /tmp/theft.json))"
echo ""

# --- 7. Public endpoints still accessible ---
echo "--- 7. Public endpoint: /health ---"
resp=$(curl -s -w "%{http_code}" -o /tmp/health.json "$BASE/health")
status="${resp: -3}"
check_status "/health → 200" "200" "$status"
echo ""

echo "================================================"
echo "  Results: $PASS passed, $FAIL failed"
echo "================================================"

# Exit codes for CI integration
if [[ "$FAIL" -gt 0 ]]; then
  exit 1
fi
exit 0
