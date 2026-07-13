#!/usr/bin/env bash
# @sk-task tenant-profile-sync#examples: Seed tenant dictionaries
#
# Updates dictionaries for tenant "default" with:
#   - 500 users  (full names)
#   - 50 departments
#   - 300 projects
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

# ---- Update tenant dictionaries ----
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
