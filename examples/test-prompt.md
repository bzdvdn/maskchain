# Test Prompts for Postman — два разных flow

## Flow A: Mask/Unmask — PII Regex (plain text)

Маскирует встроенными PII-детекторами (email, phone, SSN). Тело — `text/plain`.
`X-Mask-ID` генерируется сервером (uuid v7), клиент его не передаёт.

| Operation | Method | URL | Content-Type |
|-----------|--------|-----|--------------|
| Mask | POST | `http://localhost:8080/api/v1/shield/mask` | `text/plain` |
| Unmask | POST | `http://localhost:8080/api/v1/shield/unmask` | `text/plain` |

### Mask Request

Body (`text/plain`):
```
=== EMPLOYEE DATA EXPORT ===
Export Date: 2026-07-13
Total Records: 15

ID,FullName,Email,Phone,SSN,Position,Department,Project,Salary,StartDate,Description
EMP-001,James Smith,james.smith@acme.com,+1-555-123-4567,987-65-4321,Software Engineer,Engineering,Project-Omega,125000,2023-01-15,Senior developer working on core platform since 2023
EMP-002,Mary Johnson,mary.j@acme.com,+44-20-7946-0123,456-78-9012,Product Manager,Marketing,Project-Alpha,98000,2023-03-22,Leads product strategy for the marketing analytics suite
EMP-003,Robert Williams,rob.williams@acme.com,+1-555-987-6543,123-45-6789,DevOps Engineer,Sales,Project-Beta,112000,2023-06-10,Manages CI/CD pipelines and cloud infrastructure
EMP-004,Patricia Brown,pat.brown@acme.com,+49-30-1234-5678,789-01-2345,UX Designer,Engineering,Project-Omega,135000,2023-08-05,Designs user interfaces for internal tools and customer portal
EMP-005,John Davis,john.davis@acme.com,+81-3-5555-0101,234-56-7890,Data Analyst,HR,Project-Gamma,95000,2023-09-18,Analyzes workforce data and generates quarterly reports
EMP-006,Jennifer Garcia,j.garcia@acme.com,+61-2-5555-1234,345-67-8901,Tech Lead,Legal,Project-Delta,142000,2023-11-30,Leads technical architecture for document management system
EMP-007,Michael Rodriguez,m.rodriguez@acme.com,+1-555-234-5678,567-89-0123,Security Engineer,Finance,Project-Epsilon,118000,2024-01-12,Implements security controls and compliance monitoring
EMP-008,Linda Martinez,l.martinez@acme.com,+33-1-2345-6789,678-90-1234,Frontend Developer,Marketing,Project-Alpha,102000,2024-02-28,Builds React components for campaign management dashboard
EMP-009,David Anderson,d.anderson@acme.com,+1-555-345-6789,890-12-3456,Backend Developer,Product,Project-Zeta,128000,2024-04-15,Develops REST APIs and microservices for product catalog
EMP-010,Barbara Thomas,b.thomas@acme.com,+7-495-123-4567,901-23-4567,System Architect,Engineering,Project-Omega,145000,2024-05-20,Designs system architecture for high-availability platform
EMP-011,Richard Jackson,r.jackson@acme.com,+86-10-5555-6789,012-34-5678,QA Engineer,Support,Project-Theta,88000,2024-07-01,Responsible for automated testing and quality assurance
EMP-012,Susan White,s.white@acme.com,+1-555-456-7890,135-79-2468,Scrum Master,Operations,Project-Iota,105000,2024-08-14,Facilitates agile ceremonies and removes team impediments
EMP-013,Joseph Harris,j.harris@acme.com,+65-5555-1234,864-20-9735,Engineering Manager,Security,Project-Kappa,115000,2024-10-05,Manages engineering team and drives technical delivery
EMP-014,Jessica Clark,j.clark@acme.com,+91-22-5555-0101,753-19-8642,ML Engineer,Research,Project-Lambda,99000,2024-11-19,Develops machine learning models for predictive analytics
EMP-015,Thomas Lewis,t.lewis@acme.com,+1-555-567-8901,642-08-7531,Solutions Architect,Analytics,Project-Omega,122000,2025-01-07,Architects end-to-end solutions for client data pipelines

--- CALCULATION ---
Department Budget Report:
- Engineering: 3 employees, total salary $405,000
- Marketing: 2 employees, total salary $200,000
- Sales: 1 employee, total salary $112,000
- HR: 1 employee, total salary $95,000
- Legal: 1 employee, total salary $142,000
- Finance: 1 employee, total salary $118,000
- Product: 1 employee, total salary $128,000
- Support: 1 employee, total salary $88,000
- Operations: 1 employee, total salary $105,000
- Security: 1 employee, total salary $115,000
- Research: 1 employee, total salary $99,000
- Analytics: 1 employee, total salary $122,000

Grand Total: $1,729,000
Average Salary: $115,267

--- CONTACT INFO ---
Emergency Contact: Mary Johnson can be reached at +44-20-7946-0123 or mary.j@acme.com
Payroll Admin: Use SSN 987-65-4321 for EMP-001 payroll processing
HR Backup Contact: j.davis@acme.com or call +81-3-5555-0101
```

### Response Headers

```
X-Mask-ID: <uuid-v7>   ← сохранить для unmask, сервер сам его сгенерировал
```

### Expected Mask Response

Emails, phones, SSNs заменены на `{{p.default.N}}`. Индексы могут отличаться.

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

Проверка shield pipeline со словарями из seed-profile.sh.
Определяет имена, email'ы, должности, отделы, проекты. JSON в формате OpenAI chat completions.
Никак не связан с Flow A — это два разных эндпоинта.

### Request

```
POST http://localhost:8080/v1/chat/completions
Content-Type: application/json
X-Shield-Profile-Slug: pii-protect
Authorization: Bearer sk-test-default
```

```json
{
  "model": "gpt-4o",
  "messages": [
    {
      "role": "user",
      "content": "Please process the following employee assignment report:\n\nPROJECT ASSIGNMENTS\n====================\n\n1. James LastName42 (James.lastname42@example.com), Software Engineer from Engineering #1 is assigned to Project-42 (PRJ-042) as Lead Engineer\n2. Mary LastName99 (Mary.lastname99@example.com), Product Manager from Marketing #2 is assigned to Project-15 (PRJ-015) as Product Manager\n3. Robert LastName7 (Robert.lastname7@example.com), DevOps Engineer from Sales #3 is assigned to Project-300 (PRJ-300) as Sales Director\n4. Patricia LastName200 (Patricia.lastname200@example.com), UX Designer from Finance #4 is assigned to Project-88 (PRJ-088) as Financial Controller\n5. John LastName55 (John.lastname55@example.com), Data Analyst from Engineering #1 is assigned to Project-42 (PRJ-042) as Backend Developer\n6. Jennifer LastName150 (Jennifer.lastname150@example.com), Tech Lead from HR #5 is assigned to Project-15 (PRJ-015) as HR Business Partner\n7. Michael LastName33 (Michael.lastname33@example.com), Security Engineer from Legal #6 is assigned to Project-201 (PRJ-201) as Legal Counsel\n8. Linda LastName99 (Linda.lastname99@example.com), Frontend Developer from Marketing #2 is assigned to Project-42 (PRJ-042) as UI Developer\n9. David LastName200 (David.lastname200@example.com), Backend Developer from Product #7 is assigned to Project-88 (PRJ-088) as API Developer\n10. Susan LastName88 (Susan.lastname88@example.com), Scrum Master from Operations #8 is assigned to Project-15 (PRJ-015) as Agile Coach\n\nDEPARTMENT SUMMARY\n==================\n- Engineering #1: 2 employees assigned (James LastName42, John LastName55)\n- Marketing #2: 2 employees assigned (Mary LastName99, Linda LastName99)\n- Sales #3: 1 employee assigned (Robert LastName7)\n- Finance #4: 1 employee assigned (Patricia LastName200)\n- HR #5: 1 employee assigned (Jennifer LastName150)\n- Legal #6: 1 employee assigned (Michael LastName33)\n- Product #7: 1 employee assigned (David LastName200)\n- Operations #8: 1 employee assigned (Susan LastName88)\n\nPROJECT DETAILS\n===============\nProject-42 (PRJ-042): 3 engineers, timeline Q3-Q4 2026, budget $650,000, priority high\nProject-15 (PRJ-015): 3 cross-functional, timeline Q3 2026, budget $380,000, priority medium\nProject-300 (PRJ-300): 1 director, timeline Q4 2026, budget $180,000, priority low\nProject-88 (PRJ-088): 2 developers, timeline Q3-Q4 2026, budget $320,000, priority high\nProject-201 (PRJ-201): 1 counsel, timeline Q4 2026, budget $150,000, priority medium\n\nPlease calculate:\n1. Total headcount across all projects\n2. Total budget across all projects\n3. Department with most employees assigned\n4. List all unique positions involved in Project-42"
    }
  ]
}
```

### Expected behavior

- `X-Shield-Status: suspicious` — dictionary entries detected from all 3 dictionaries:
  - **users**: names (`James LastName42`, `Mary LastName99`, etc.), emails, positions (`Software Engineer`, `Product Manager`, etc.)
  - **departments**: `Engineering #1`, `Marketing #2`, etc.
  - **projects**: `Project-42`, `PRJ-042`, `Project-15`, `PRJ-015`, etc.
- Request continues to LLM provider (or 502 if no provider — expected)
- Incident created for each detected entry

---

## Postman Setup Tips

1. Create Collection "MaskChain"
2. **Mask**: `POST http://localhost:8080/api/v1/shield/mask` with `Content-Type: text/plain` → body from section 1
3. Capture `X-Mask-ID` from response
4. **Unmask**: `POST http://localhost:8080/api/v1/shield/unmask?mask_ids=<ID>` with masked body
5. **Shield Scan**: `POST http://localhost:8080/v1/chat/completions` with JSON from section 3 + required headers
