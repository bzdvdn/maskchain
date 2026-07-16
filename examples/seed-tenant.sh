#!/usr/bin/env bash
# @sk-task tenant-profile-sync#examples: Seed tenant dictionaries
#
# Updates dictionaries + PIIConfig for tenant "default" with:
#   - 500 users  (full names)
#   - 50 departments
#   - 300 projects
#   - PII rules (email, phone, SSN via PIIConfig)
#
# Usage: ./seed-tenant.sh [admin_url] [api_key]
#   admin_url  — default: http://localhost:8082
#   api_key    — default: sk-test-default

set -euo pipefail

ADMIN_URL="${1:-http://localhost:8082}"
API_KEY="${2:-sk-test-default}"
AUTH="Authorization: Bearer ${API_KEY}"

echo "=== MaskChain Tenant Dictionary Seed ==="
echo "Admin: ${ADMIN_URL}"
echo ""

# ---- Wait for admin ----
echo "Waiting for admin API..."
for i in $(seq 1 30); do
  if curl -sf "${ADMIN_URL}/health" >/dev/null 2>&1; then
    echo "Admin ready."
    break
  fi
  sleep 2
done

# ---- Load dictionaries from JSON files ----
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
DATA_DIR="${SCRIPT_DIR}/data"

echo "Loading users from data/users.json..."
USERS_JSON=$(cat "${DATA_DIR}/users.json")

echo "Loading departments from data/departments.json..."
DEPT_JSON=$(cat "${DATA_DIR}/departments.json")

echo "Loading projects from data/projects.json..."
PROJ_JSON=$(cat "${DATA_DIR}/projects.json")

# ---- Build dictionaries payload ----
DICT_JSON=$(cat <<EOF
{
  "dictionaries": [
    {
      "name": "users",
      "entries": ${USERS_JSON},
      "match_mode": "exact"
    },
    {
      "name": "departments",
      "entries": ${DEPT_JSON},
      "match_mode": "exact"
    },
    {
      "name": "projects",
      "entries": ${PROJ_JSON},
      "match_mode": "exact"
    }
  ]
}
EOF
)

# ---- Build PIIConfig payload ----
# PII rules are per-tenant via PIIConfig (email, phone, SSN).
# Note: requires Admin API support for pii_config field.
PII_JSON=$(cat <<EOF
{
  "name": "Default Tenant",
  "auth_header": "Authorization",
  "api_keys": ["sk-test-default"],
  "pii_config": {
    "enabled": true,
    "default_action": "mask",
    "rules": [
      {"label": "email", "type": "regex", "pattern": "EMAIL", "action": "block"},
      {"label": "phone", "type": "regex", "pattern": "PHONE", "action": "block"},
      {"label": "ssn", "type": "regex", "pattern": "SSN", "action": "block"}
    ]
  }
}
EOF
)

# ---- Update tenant PIIConfig FIRST ----
# Must come BEFORE dictionaries: PUT /api/v1/tenants/:slug updates ALL fields,
# and the payload does not include dictionaries (to avoid duplicating 900+ entries).
echo ""
echo "Updating PIIConfig for tenant 'default'..."

HTTP_CODE_PII=$(curl -s -o /tmp/seed-pii-response.json -w "%{http_code}" \
  -X PUT "${ADMIN_URL}/api/v1/tenants/default" \
  -H "Content-Type: application/json" \
  -H "${AUTH}" \
  -d "${PII_JSON}" 2>/dev/null || echo "000")

if [ "${HTTP_CODE_PII}" = "200" ]; then
  echo "Tenant PIIConfig updated."
else
  echo "WARN: Failed to update PIIConfig (HTTP ${HTTP_CODE_PII})"
  echo "       Admin API may not support pii_config field yet."
  cat /tmp/seed-pii-response.json 2>/dev/null || true
fi

# ---- Update tenant dictionaries SECOND ----
# Uses dedicated /dictionaries endpoint which only touches dictionaries + updated_at,
# so it does NOT overwrite pii_config set above.
echo ""
echo "Updating dictionaries for tenant 'default'..."

HTTP_CODE=$(curl -s -o /tmp/seed-dict-response.json -w "%{http_code}" \
  -X PUT "${ADMIN_URL}/api/v1/tenants/default/dictionaries" \
  -H "Content-Type: application/json" \
  -H "${AUTH}" \
  -d "${DICT_JSON}" 2>/dev/null || echo "000")

if [ "${HTTP_CODE}" = "200" ]; then
  echo "Tenant dictionaries updated."
else
  echo "FAILED to update dictionaries (HTTP ${HTTP_CODE})"
  cat /tmp/seed-dict-response.json
  exit 1
fi

echo ""
echo "=== Seed complete ==="
echo ""
echo "Next steps:"
echo "  1. Open examples/test-prompt.md for Postman test prompts"
echo "  2. POST to http://localhost:8080/api/v1/shield/mask with Authorization: Bearer sk-test-default"
echo "  3. Use X-Mask-ID header from response to unmask via /api/v1/shield/unmask"
echo "  4. PII rules (email/phone/SSN) are configured per-tenant via PIIConfig"
