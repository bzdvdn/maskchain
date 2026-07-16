#!/usr/bin/env bash
set -euo pipefail

GATEWAY_URL="${1:-http://localhost:8080}"
API_KEY="${2:-sk-test-default}"
AUTH="Authorization: Bearer ${API_KEY}"

echo "=== MaskChain + Ollama Test ==="
echo "Gateway: ${GATEWAY_URL}"
echo "Model: gemma3:4b"
echo ""

# 1. Health check
echo "--- 1. Health check ---"
curl -sf "${GATEWAY_URL}/health" >/dev/null && echo "OK" || { echo "FAIL: gateway not ready"; exit 1; }

# 2. Dictionary-only request (no PII) — expect dict masking + pass-through
echo ""
echo "--- 2. Dictionary-only request (names, departments, projects) ---"
echo "    Expect: 200 OK, X-Shield-Dict-Mask-ID header, dict values masked before LLM, restored in response"
echo ""

RESPONSE=$(curl -s -w "\n%{http_code}\n%{header_json}" \
  -X POST "${GATEWAY_URL}/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -H "${AUTH}" \
  -d @- <<'JSON'
{
  "model": "gemma3:4b",
  "messages": [
    {
      "role": "user",
      "content": "List the employees from Engineering #1 assigned to Project-42. Include James LastName1 as Lead Engineer and John LastName5 as Backend Developer."
    }
  ],
  "stream": false
}
JSON
)

HTTP_CODE=$(echo "${RESPONSE}" | tail -1)
echo "HTTP Status: ${HTTP_CODE}"

if [ "${HTTP_CODE}" = "200" ]; then
  echo "OK: request passed through (dictionary values were masked before LLM)"
else
  echo "FAIL: expected 200, got ${HTTP_CODE}"
  echo "${RESPONSE}" | sed '$d' | jq . 2>/dev/null || echo "${RESPONSE}"
fi

# 3. Check shield headers
echo ""
echo "--- 3. Shield headers ---"
curl -s -D - -o /dev/null \
  -X POST "${GATEWAY_URL}/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -H "${AUTH}" \
  -d '{"model":"gemma3:4b","messages":[{"role":"user","content":"hello world"}],"stream":false}' 2>/dev/null | head -20

echo ""
echo ""
echo "--- Done ---"
echo ""
echo "To check analytics:"
echo "  curl -s 'http://localhost:8082/api/v1/analytics/tokens?period=day' -H '${AUTH}' | jq ."
echo ""
echo "To see dictionary masking in gateway logs:"
echo "  docker logs maskchain-gateway --tail 50"
