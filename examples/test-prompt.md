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
=== EMPLOYEE DATA EXPORT ===
Export Date: 2026-07-13
Total Records: 10

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

--- CALCULATION ---
Department Budget Report:
- Engineering #1: 2 employees, total salary $220,000
- Marketing #2: 2 employees, total salary $200,000
- Sales #3: 1 employee, total salary $112,000
- Finance #4: 1 employee, total salary $135,000
- HR #5: 1 employee, total salary $142,000
- Operations #6: 1 employee, total salary $145,000
- Legal #7: 1 employee, total salary $118,000
- Product #8: 1 employee, total salary $128,000

Grand Total: $1,200,000

--- CONTACT INFO ---
Emergency Contact: Mary LastName2 can be reached at +44-20-7946-0123 or Mary.lastname2@example.com
Payroll Admin: Use SSN 987-65-4321 for EMP-001 payroll processing
HR Backup Contact: John.lastname5@example.com or call +81-3-5555-0101
```

### Response Headers

```
X-Mask-ID: <uuid-v7>   ← сохранить для unmask, сервер сам его сгенерировал
```

### Expected Mask Response

Emails, phones, SSNs заменены на `{{p.default.N}}`, словарные значения (имена, должности, отделы, проекты) — на `{{dict.<uuid>.<N>}}`. Индексы могут отличаться.

### Unmask Request

```
POST http://localhost:8080/api/v1/shield/unmask?mask_ids=<X-Mask-ID-из-шага-1>
Content-Type: text/plain
```

Body: замаскированный текст из ответа на шаге Mask (с `{{p.default.N}}`).

### Expected Unmask Response

Исходный текст восстановлен.

---

## Flow B: Shield Scan — Dictionary Matching (JSON)

Проверка shield pipeline со словарями из seed-tenant.sh.
Определяет имена, email'ы, должности, отделы, проекты. JSON в формате OpenAI chat completions.
Никак не связан с Flow A — это два разных эндпоинта.

Тенант определяется по API-ключу из заголовка `Authorization: Bearer <key>`.
PII-правила (PIIConfig) и словари загружаются из тенанта.

### Request

```
POST http://localhost:8080/v1/chat/completions
Content-Type: application/json
Authorization: Bearer sk-test-default
```

```json
{
  "model": "gpt-4o",
  "messages": [
    {
      "role": "user",
      "content": "Please process the following employee assignment report:\n\nPROJECT ASSIGNMENTS\n====================\n\n1. James LastName1 (James.lastname1@example.com), Software Engineer from Engineering #1 is assigned to Project-42 (PRJ-042) as Lead Engineer\n2. Mary LastName2 (Mary.lastname2@example.com), Senior Developer from Marketing #2 is assigned to Project-15 (PRJ-015) as Product Manager\n3. Robert LastName3 (Robert.lastname3@example.com), Product Manager from Sales #3 is assigned to Project-300 (PRJ-300) as Sales Director\n4. Patricia LastName4 (Patricia.lastname4@example.com), UX Designer from Finance #4 is assigned to Project-88 (PRJ-088) as Financial Controller\n5. John LastName5 (John.lastname5@example.com), Data Analyst from Engineering #1 is assigned to Project-42 (PRJ-042) as Backend Developer\n6. Jennifer LastName6 (Jennifer.lastname6@example.com), DevOps Engineer from HR #5 is assigned to Project-15 (PRJ-015) as HR Business Partner\n7. Michael LastName7 (Michael.lastname7@example.com), QA Engineer from Legal #7 is assigned to Project-201 (PRJ-201) as Legal Counsel\n8. Linda LastName8 (Linda.lastname8@example.com), Tech Lead from Marketing #2 is assigned to Project-42 (PRJ-042) as UI Developer\n9. David LastName9 (David.lastname9@example.com), Frontend Developer from Product #8 is assigned to Project-88 (PRJ-088) as API Developer\n10. Barbara LastName10 (Barbara.lastname10@example.com), Backend Developer from Operations #6 is assigned to Project-15 (PRJ-015) as Agile Coach\n\nDEPARTMENT SUMMARY\n==================\n- Engineering #1: 2 employees assigned (James LastName1, John LastName5)\n- Marketing #2: 2 employees assigned (Mary LastName2, Linda LastName8)\n- Sales #3: 1 employee assigned (Robert LastName3)\n- Finance #4: 1 employee assigned (Patricia LastName4)\n- HR #5: 1 employee assigned (Jennifer LastName6)\n- Operations #6: 1 employee assigned (Barbara LastName10)\n- Legal #7: 1 employee assigned (Michael LastName7)\n- Product #8: 1 employee assigned (David LastName9)\n\nPROJECT SUMMARY\n===============\n- Project-42 (PRJ-042): 3 employees (James LastName1, John LastName5, Linda LastName8)\n- Project-15 (PRJ-015): 2 employees (Mary LastName2, Jennifer LastName6)\n- Project-88 (PRJ-088): 2 employees (Patricia LastName4, David LastName9)\n- Project-300 (PRJ-300): 1 employee (Robert LastName3)\n- Project-201 (PRJ-201): 1 employee (Michael LastName7)"
    }
  ]
}
```

### Expected behavior

- `X-Shield-Status: suspicious` — dictionary entries detected from all 3 dictionaries:
  - **users**: names (`James LastName1`, `Mary LastName2`, etc.), emails (`James.lastname1@example.com`, etc.), positions (`Software Engineer`, `Senior Developer`, etc.)
  - **departments**: `Engineering #1`, `Marketing #2`, etc.
  - **projects**: `Project-42`, `PRJ-042`, `Project-15`, `PRJ-015`, etc.
- Request body modified before sending to provider: dictionary values заменены на `{{dict.<uuid>.<N>}}` (предотвращение утечки)
- Incident created for each detected entry

---

## Postman Setup Tips

1. Create Collection "MaskChain"
2. **Mask**: `POST http://localhost:8080/api/v1/shield/mask` with `Authorization: Bearer sk-test-default` and `Content-Type: text/plain` → body from section 1 (PII + словарные значения будут замаскированы)
3. Capture `X-Mask-ID` from response
4. **Unmask**: `POST http://localhost:8080/api/v1/shield/unmask?mask_ids=<ID>` with masked body
5. **Shield Scan**: `POST http://localhost:8080/v1/chat/completions` with JSON from section 3 + required headers (словарные значения будут замаскированы перед отправкой к LLM)
