# Postman collection

Import `MaskChain.postman_collection.json` into Postman.

## Variables

| Variable | Default | Notes |
|---|---|---|
| `baseUrl` | `http://localhost:8080` | Gateway |
| `adminUrl` | `http://localhost:9090` | Admin |
| `apiKey` | `sk-test-default` | Tenant key |
| `adminToken` | *(set by Login)* | Captured automatically |
| `maskId` | *(set by Mask)* | Captured from the `mask-id` response header |
| `model` | `llama3.2` | Change to `gpt-4o-mini` for the OpenAI quickstart |

## Order

1. **Admin → Login** — stores `adminToken`.
2. **Shield → Mask plain text** — stores `maskId`, then run **Unmask**.
3. **Data plane → Chat completion** — check `X-Shield-Status` in the headers.
4. **Admin → Create virtual key / budget / List providers**.

Prefer curl? The same calls are in [`../cookbook/`](../cookbook/).
