#!/usr/bin/env bash
set -euo pipefail

# @sk-test 118-api-consistency#T4.2: Integration smoke test for API consistency (AC-007, SC-002)
#
# Usage:
#   SERVER_URL=http://localhost:8080 ./test/integration/api-consistency.sh
#
# Requires: curl, jq
# Starts a Go test server if SERVER_URL is not set (uses ./test/integration/testserver)

SERVER_URL="${SERVER_URL:-}"
PASS=0
FAIL=0

require() {
    if ! command -v "$1" &>/dev/null; then
        echo "SKIP: $1 not found"
        exit 0
    fi
}

cleanup() {
    if [ -n "${TEST_PID:-}" ]; then
        kill "$TEST_PID" 2>/dev/null || true
        wait "$TEST_PID" 2>/dev/null || true
    fi
}
trap cleanup EXIT

check_envelope() {
    local method="$1" path="$2" desc="$3" expect_status="$4" extra=()
    shift 4
    while [ $# -gt 0 ]; do
        extra+=("$1")
        shift
    done

    local url="${SERVER_URL}${path}"
    local code body

    if [ "$method" = "GET" ]; then
        body=$(curl -s -w "\n%{http_code}" "${extra[@]}" "$url")
    else
        body=$(curl -s -w "\n%{http_code}" -X "$method" "${extra[@]}" "$url")
    fi

    code=$(echo "$body" | tail -1)
    body=$(echo "$body" | sed '$d')

    if [ "$code" != "$expect_status" ]; then
        echo "FAIL [$desc] expected status $expect_status got $code"
        echo "  Response: $(echo "$body" | head -c 200)"
        FAIL=$((FAIL + 1))
        return
    fi

    if echo "$body" | jq -e '.data != null or .error != null' &>/dev/null; then
        echo "PASS [$desc] status=$code envelope OK"
        PASS=$((PASS + 1))
    elif [ "$expect_status" = "204" ]; then
        echo "PASS [$desc] status=204 no body"
        PASS=$((PASS + 1))
    elif echo "$body" | jq -e '.choices != null' &>/dev/null; then
        echo "PASS [$desc] status=$code proxy body (non-envelope)"
        PASS=$((PASS + 1))
    else
        echo "FAIL [$desc] unexpected body format"
        echo "  Response: $(echo "$body" | head -c 200)"
        FAIL=$((FAIL + 1))
    fi
}

require curl
require jq

echo "=== API Consistency Smoke Test ==="
echo ""

# ---- Profiles ----
check_envelope GET "/api/v1/profiles" "GET /api/v1/profiles" 200
check_envelope POST "/api/v1/profiles" "POST /api/v1/profiles" 201 \
    -H "Content-Type: application/json" -d '{"slug":"test","name":"Test","description":"test"}'
check_envelope GET "/api/v1/profiles/test" "GET /api/v1/profiles/test" 200
check_envelope PUT "/api/v1/profiles/test" "PUT /api/v1/profiles/test" 200 \
    -H "Content-Type: application/json" -d '{"name":"Updated"}'
check_envelope DELETE "/api/v1/profiles/test" "DELETE /api/v1/profiles/test" 204

# ---- Incidents ----
check_envelope GET "/api/v1/incidents" "GET /api/v1/incidents" 200
check_envelope GET "/api/v1/incidents/export?format=json" "GET /api/v1/incidents/export" 200

# ---- Shield ----
check_envelope POST "/api/v1/shield/mask" "POST /api/v1/shield/mask" 200 \
    -H "Content-Type: text/plain" -d "test text"

# ---- Redirects ----
code=$(curl -s -o /dev/null -w "%{http_code}" "${SERVER_URL}/v1/chat/completions" -X POST)
if [ "$code" = "301" ]; then
    echo "PASS [301 /v1/chat/completions -> /api/v1/]"
    PASS=$((PASS + 1))
else
    echo "FAIL [301 /v1/chat/completions] expected 301 got $code"
    FAIL=$((FAIL + 1))
fi

code=$(curl -s -o /dev/null -w "%{http_code}" "${SERVER_URL}/v1/completions" -X POST)
if [ "$code" = "301" ]; then
    echo "PASS [301 /v1/completions -> /api/v1/]"
    PASS=$((PASS + 1))
else
    echo "FAIL [301 /v1/completions] expected 301 got $code"
    FAIL=$((FAIL + 1))
fi

# ---- 404 envelope ----
body=$(curl -s "${SERVER_URL}/api/v1/nonexistent")
if echo "$body" | jq -e '.error != null and .data == null' &>/dev/null; then
    echo "PASS [404 envelope] error envelope for unknown route"
    PASS=$((PASS + 1))
else
    echo "FAIL [404 envelope] expected error envelope"
    echo "  Response: $(echo "$body" | head -c 200)"
    FAIL=$((FAIL + 1))
fi

# ---- Health endpoints (no envelope) ----
code=$(curl -s -o /dev/null -w "%{http_code}" "${SERVER_URL}/health")
if [ "$code" = "200" ]; then
    echo "PASS [/health] returns 200 (no envelope)"
    PASS=$((PASS + 1))
else
    echo "FAIL [/health] expected 200 got $code"
    FAIL=$((FAIL + 1))
fi

echo ""
echo "=== Results: $PASS passed, $FAIL failed ==="

if [ "$FAIL" -gt 0 ]; then
    exit 1
fi
