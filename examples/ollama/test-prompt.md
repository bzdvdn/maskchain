# Test Prompts for Postman — Ollama + Content Shield

## Flow: Chat Completion с PII и словарями через Ollama

Проверяет shield pipeline: tenant `default` определяется по API-ключу, его PII-правила и словари (users, departments, projects) применяются к запросу перед отправкой к локальной Ollama (gemma3:4b).

| Метод | URL | Content-Type | Auth |
|-------|-----|--------------|------|
| POST | `http://localhost:8080/v1/chat/completions` | `application/json` | `Authorization: Bearer sk-test-default` |

---

### Request 1: Базовая проверка (без PII, без словарей)

Убедиться что Ollama отвечает через gateway:

```json
{
  "model": "gemma3:4b",
  "messages": [
    {
      "role": "user",
      "content": "Say hello in 5 words"
    }
  ],
  "stream": false
}
```

**Ожидается:** 200 OK, тело ответа с `choices[0].message.content` от gemma3:4b.
**Response Headers:** нет shield-заголовков (clean запрос).

---

### Request 2: Словарные данные БЕЗ PII — проверка маскировки словарей

**Самый важный запрос.** Проверяет что shield **заменяет** имена, отделы и проекты из словарей тенанта на `{{dict.*}}` перед отправкой к Ollama, а в ответе восстанавливает оригиналы.

В этом запросе **нет** email, телефонов или SSN — только словарные значения (имена, отделы, проекты). PII-детекторы не срабатывают, запрос доходит до Ollama, но словари маскируются.

```json
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
```

**Ожидаемое поведение (под капотом):**
1. Shield находит в тексте: `Engineering #1`, `Project-42`, `James LastName1`, `John LastName5`, `Lead Engineer`, `Backend Developer` — все эти значения есть в словарях тенанта (`departments`, `projects`, `users`)
2. **Перед отправкой к Ollama** эти значения заменяются на `{{dict.<id>.0}}`, `{{dict.<id>.1}}`, ...
3. Ollama получает уже замаскированный запрос (например: *"List the employees from {{dict.abc.0}} assigned to {{dict.abc.1}}..."*)
4. Ollama генерирует ответ с плейсхолдерами
5. **Перед отправкой клиенту** shield восстанавливает оригиналы

**Headers в ответе:**
```
X-Shield-Status: suspicious      ← словари обнаружены
X-Shield-Dict-Mask-ID: <id>     ← ID маскировки словарей
```

**Тело ответа:** содержит оригинальные имена, отделы и проекты (восстановлены из плейсхолдеров).

---

### Request 3: PII + словари — проверка маскировки PII и словарей

Запрос содержит email, телефон, SSN И словарные данные (проекты, отделы). Shield заменяет все на плейсхолдеры перед отправкой к Ollama:

```json
{
  "model": "gemma3:4b",
  "messages": [
    {
      "role": "user",
      "content": "My email is james@example.com, my phone is +1-555-123-4567, and my SSN is 987-65-4321. I work on Project-42 in Engineering #1."
    }
  ],
  "stream": false
}
```

**Ожидаемое поведение (под капотом):**
1. Shield находит словарные совпадения (`Engineering #1`, `Project-42`) → заменяет на `{{dict.*}}`
2. PII-детектор находит email, телефон, SSN → заменяет на `{{pii.email.0}}`, `{{pii.phone.0}}`, `{{pii.ssn.0}}`
3. Ollama получает полностью замаскированный запрос (без PII, без словарных имён)
4. В ответе все плейсхолдеры восстанавливаются — клиент видит оригинальные данные

**Headers в ответе:**
```
X-Shield-Status: suspicious      ← PII обнаружен, но замаскирован
X-Shield-Dict-Mask-ID: <id>     ← ID маскировки словарей
```

**Тело ответа:** содержит оригинальные email, телефон, SSN, проекты, отделы (восстановлены из плейсхолдеров).

---

### Request 4: Streaming (без PII)

Проверяет streaming через Ollama:

```json
{
  "model": "gemma3:4b",
  "messages": [
    {
      "role": "user",
      "content": "Write a short poem about AI safety"
    }
  ],
  "stream": true
}
```

**Ожидается:** SSE chunks (`data: {...}`), последний chunk `data: [DONE]`.

---

### Postman Setup Tips

1. Create Collection "MaskChain Ollama"
2. Set Collection Variables:
   - `base_url`: `http://localhost:8080`
   - `auth`: `Bearer sk-test-default`
3. Add Header `Authorization: {{auth}}` to all requests
4. Add Header `Content-Type: application/json` to all requests

**Порядок тестирования:**

| # | Запрос | Что проверяем |
|---|--------|---------------|
| 1 | Базовая проверка | Ollama работает через gateway |
| 2 | Словари без PII | **Словарные значения заменяются на `{{dict.*}}`** → восстанавливаются в ответе |
| 3 | PII + словари | **PII и словари заменяются на `{{pii.*}}` и `{{dict.*}}`** → восстанавливаются в ответе |
| 4 | Streaming | SSE chunks работают |

**Как увидеть что словари и PII реально заменились:** включить debug-логи gateway (`docker logs maskchain-gateway`) — в логе будет видно тело запроса к Ollama с `{{dict.*}}` и `{{pii.*}}` плейсхолдерами.
