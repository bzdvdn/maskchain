#!/usr/bin/env bash
# @sk-task 102-profile-cache#examples: Seed profile with test dictionaries
#
# Creates/updates profile "pii-protect" with:
#   - 500 users  (full names)
#   - 50 departments
#   - 300 projects
#
# Usage: ./seed-profile.sh [admin_url] [api_key]
#   admin_url  — default: http://localhost:8081
#   api_key    — default: sk-test-default

set -euo pipefail

ADMIN_URL="${1:-http://localhost:8082}"
API_KEY="${2:-sk-test-default}"
AUTH="Authorization: Bearer ${API_KEY}"

echo "=== MaskChain Profile Seed ==="
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

# ---- Build profile JSON (no preprocessors) ----
PROFILE_JSON=$(cat <<EOF
{
  "slug": "pii-protect",
  "name": "PII Protection",
  "description": "Protects PII: user names, departments, projects via dictionary matching",
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

# ---- Create or Update ----
echo ""
echo "Creating/updating profile 'pii-protect'..."

HTTP_CODE=$(curl -s -o /tmp/seed-profile-response.json -w "%{http_code}" \
  -X POST "${ADMIN_URL}/api/v1/profiles" \
  -H "Content-Type: application/json" \
  -H "${AUTH}" \
  -d "${PROFILE_JSON}" 2>/dev/null || echo "000")

if [ "${HTTP_CODE}" = "201" ]; then
  echo "Profile created."
elif [ "${HTTP_CODE}" = "409" ]; then
  echo "Profile exists, updating..."
  HTTP_CODE=$(curl -s -o /tmp/seed-profile-response.json -w "%{http_code}" \
    -X PUT "${ADMIN_URL}/api/v1/profiles/pii-protect" \
    -H "Content-Type: application/json" \
    -H "${AUTH}" \
    -d "${PROFILE_JSON}" 2>/dev/null || echo "000")
  if [ "${HTTP_CODE}" = "200" ]; then
    echo "Profile updated."
  else
    echo "FAILED to update profile (HTTP ${HTTP_CODE})"
    cat /tmp/seed-profile-response.json
    exit 1
  fi
else
  echo "FAILED to create profile (HTTP ${HTTP_CODE})"
  cat /tmp/seed-profile-response.json
  exit 1
fi

echo ""
echo "=== Seed complete ==="
echo ""
echo "Next steps:"
echo "  1. Open examples/test-prompt.md for Postman test prompts"
echo "  2. POST to http://localhost:8080/api/v1/shield/mask with X-Shield-Profile-Slug: pii-protect"
echo "  3. Use X-Mask-ID header from response to unmask via /api/v1/shield/unmask"
