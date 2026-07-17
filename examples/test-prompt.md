# Test Prompts for Postman — два разных flow

## Flow A: Mask/Unmask — PII Regex + Dictionary (plain text)

Маскирует PII-детекторами (email, phone, SSN) по правилам из **tenant.PIIConfig** и словарными значениями (имена, должности, отделы, проекты) из тенанта. Тело — `text/plain`.

PII-правила per-tenant: email→block, phone→block, ssn→block (настраивается в `config.yaml` или через admin API).

`X-Mask-ID` генерируется сервером (uuid v7), клиент его не передаёт.

| Operation | Method | URL | Content-Type | Auth |
|-----------|--------|-----|--------------|------|
| Mask | POST | `http://localhost:8080/api/v1/shield/mask` | `text/plain` | `Authorization: Bearer sk-test-default` |
| Unmask | POST | `http://localhost:8080/api/v1/shield/unmask` | `text/plain` | — |

### Mask Request

Body (`text/plain`):
```
=== EMPLOYEE DIRECTORY (CONFIDENTIAL) ===
Export: 2026-07-13 | Records: 10

ID,FullName,Email,Phone,SSN,Position,Department,Project,Salary
EMP-001,James LastName1,James.lastname1@example.com,+1-555-123-4567,987-65-4321,Software Engineer,Engineering #1,Project-42,125000
EMP-002,Mary LastName2,Mary.lastname2@example.com,+44-20-7946-0123,456-78-9012,Senior Developer,Marketing #2,Project-15,98000
EMP-003,Robert LastName3,Robert.lastname3@example.com,+1-555-987-6543,123-45-6789,Product Manager,Sales #3,Project-300,112000
EMP-004,Patricia LastName4,Patricia.lastname4@example.com,+49-30-1234-5678,789-01-2345,UX Designer,Finance #4,Project-88,135000
EMP-005,John LastName5,John.lastname5@example.com,+81-3-5555-0101,234-56-7890,Data Analyst,Engineering #1,Project-42,95000
EMP-006,Jennifer LastName6,Jennifer.lastname6@example.com,+61-2-5555-1234,345-67-8901,DevOps Engineer,HR #5,Project-15,142000
EMP-007,Michael LastName7,Michael.lastname7@example.com,+1-555-234-5678,567-89-0123,QA Engineer,Legal #7,Project-201,118000
EMP-008,Linda LastName8,Linda.lastname8@example.com,+33-1-2345-6789,678-90-1234,Tech Lead,Marketing #2,Project-42,102000
EMP-009,David LastName9,David.lastname9@example.com,+1-555-345-6789,890-12-3456,Frontend Developer,Product #8,Project-88,128000
EMP-010,Barbara LastName10,Barbara.lastname10@example.com,+7-495-123-4567,901-23-4567,Backend Developer,Operations #6,Project-15,145000

--- ANALYSIS REQUEST (simulates what a user would ask an LLM) ---
Based on the table above, answer:
1. Who is the highest paid employee? Show their name, email, phone, and salary.
2. List all employees sorted by salary descending (top 3).
3. Which department has the highest total salary cost?
4. What is the average salary across all employees?
```

### Response Headers

```
X-Mask-ID: <uuid-v7>   ← сохранить для unmask, сервер сам его сгенерировал
```

### Expected Mask Response

- Emails, phones, SSNs → `[[pii.default.N]]` (заблокированы, не восстанавливаются)
- Имена (James LastName1, Mary LastName2, etc.) → `[MASK_USERS_<maskId>.<N>]`
- Должности (Software Engineer, Senior Developer, etc.) → `[MASK_POSITIONS_<maskId>.<N>]`
- Отделы (Engineering #1, Marketing #2, etc.) → `[MASK_DEPARTMENTS_<maskId>.<N>]`
- Проекты (Project-42, Project-15, etc.) → `[MASK_PROJECTS_<maskId>.<N>]`

Индексы `N` могут отличаться.

### Unmask Request

```
POST http://localhost:8080/api/v1/shield/unmask?mask_ids=<X-Mask-ID-из-шага-1>
Content-Type: text/plain
```

Body: замаскированный текст из ответа на шаге Mask (например, `[MASK_USERS_<maskId>.0]`, `[MASK_DEPARTMENTS_<maskId>.1]`, etc.).

### Expected Unmask Response

Исходный текст восстановлен (все `[MASK_CATEGORY_<maskId>.<N>]` заменены обратно на оригиналы). PII (`[[pii.default.N]]`) остаются заблокированными — они не восстанавливаются.

---

## Flow B: Shield Scan — Dictionary Matching (JSON)

Проверка shield pipeline со словарями из seed-tenant.sh.
Определяет имена, email'ы, должности, отделы, проекты. JSON в формате OpenAI chat completions.
Никак не связан с Flow A — это два разных эндпоинта.

Тенант определяется по API-ключу из заголовка `Authorization: Bearer <key>`.
PII-правила (PIIConfig) и словари загружаются из тенанта.

### Request

Выберите `model` из настроенных в `examples/ollama/config.yaml` → `routing.rules[].routes[].model`:
- Ollama: `gemma3:4b` или `llama3.2`
- Mistral: `open-mistral-nemo`, `mistral-small-latest`, etc.

```
POST http://localhost:8080/v1/chat/completions
Content-Type: application/json
Authorization: Bearer sk-test-default
```

```json
{
  "model": "gemma3:4b",
  "messages": [
    {
      "role": "user",
      "content": "I have an employee database dump in CSV format below. Please analyze it and give me a clear answer with a table.\n\n```\nID,FullName,Email,Phone,SSN,Position,Department,Project,Salary\nEMP-001,James LastName1,James.lastname1@example.com,+1-555-123-4567,987-65-4321,Software Engineer,Engineering #1,Project-42,125000\nEMP-002,Mary LastName2,Mary.lastname2@example.com,+44-20-7946-0123,456-78-9012,Senior Developer,Marketing #2,Project-15,98000\nEMP-003,Robert LastName3,Robert.lastname3@example.com,+1-555-987-6543,123-45-6789,Product Manager,Sales #3,Project-300,112000\nEMP-004,Patricia LastName4,Patricia.lastname4@example.com,+49-30-1234-5678,789-01-2345,UX Designer,Finance #4,Project-88,135000\nEMP-005,John LastName5,John.lastname5@example.com,+81-3-5555-0101,234-56-7890,Data Analyst,Engineering #1,Project-42,95000\nEMP-006,Jennifer LastName6,Jennifer.lastname6@example.com,+61-2-5555-1234,345-67-8901,DevOps Engineer,HR #5,Project-15,142000\nEMP-007,Michael LastName7,Michael.lastname7@example.com,+1-555-234-5678,567-89-0123,QA Engineer,Legal #7,Project-201,118000\nEMP-008,Linda LastName8,Linda.lastname8@example.com,+33-1-2345-6789,678-90-1234,Tech Lead,Marketing #2,Project-42,102000\nEMP-009,David LastName9,David.lastname9@example.com,+1-555-345-6789,890-12-3456,Frontend Developer,Product #8,Project-88,128000\nEMP-010,Barbara LastName10,Barbara.lastname10@example.com,+7-495-123-4567,901-23-4567,Backend Developer,Operations #6,Project-15,145000\n```\n\nPlease answer:\n1. Who is the highest paid employee? Include their name, email, phone number, and salary.\n2. List all employees sorted by salary descending (top 3).\n3. What is the average salary?\n4. Which department has the highest total salary expenditure?\n5. Do any employees share the same project? If so, which project and who?"
    }
  ]
}
```

### Expected behavior

- `X-Shield-Status: suspicious` — dictionary entries detected from all 3 dictionaries:
  - **users**: names (`James LastName1`, `Mary LastName2`, etc.), emails (`James.lastname1@example.com`, etc.), positions (`Software Engineer`, `Senior Developer`, etc.)
  - **departments**: `Engineering #1`, `Marketing #2`, etc.
  - **projects**: `Project-42`, `Project-15`, etc.
- Request body modified before sending to provider: имена → `[MASK_USERS_<maskId>.<N>]`, должности → `[MASK_POSITIONS_<maskId>.<N>]`, отделы → `[MASK_DEPARTMENTS_<maskId>.<N>]`, проекты → `[MASK_PROJECTS_<maskId>.<N>]`
- PII (email, phone, SSN) → `[[pii.default.N]]` (заблокированы, не отправляются провайдеру)
- Incident created for each detected entry
- **LLM ответ** должен содержать те же placeholders (модель видит только их, не оригиналы). Затем **unmask middleware** заменяет `[MASK_CATEGORY_<maskId>.<N>]` обратно перед отправкой клиенту. PII остаются заблокированными.

---

## Postman Setup Tips

1. Create Collection "MaskChain"
2. **Mask** (`Flow A`): `POST http://localhost:8080/api/v1/shield/mask` with `Authorization: Bearer sk-test-default` and `Content-Type: text/plain` → body from section 1 (PII заблокируются, dict-значения заменятся на placeholders вида `[MASK_CATEGORY_<maskId>.<N>]`)
3. Capture `X-Mask-ID` from response
4. **Unmask** (`Flow A`): `POST http://localhost:8080/api/v1/shield/unmask?mask_ids=<ID>` with masked body (placeholders восстановятся, PII останутся `[[pii.default.N]]`)
5. **Shield Scan** (`Flow B`): `POST http://localhost:8080/v1/chat/completions` with JSON from section 3 + required headers (dict-значения заменятся на placeholders перед отправкой к LLM; после ответа LLM — автоматический unmask placeholders обратно, PII невосстановимы)
6. **Проверка unmask в ответе LLM**: убедитесь, что ответ содержит оригинальные имена/отделы/проекты (а не `[MASK_USERS_xxx.0]`). Если видите placeholders — unmask в response path не сработал.
