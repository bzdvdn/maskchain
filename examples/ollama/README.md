# MaskChain + Ollama: локальное тестирование

Запуск gateway с локальной Ollama (gemma3:4b) для проверки Content Shield, маскировки PII и analytics.

## Требования

- Docker + Docker Compose
- `ollama serve` с моделью `gemma3:4b` (или любой другой)

## Быстрый старт

```bash
# 1. Убедись что ollama запущен и модель загружена
ollama serve
ollama pull gemma3:4b

# 2. Запусти инфраструктуру (PG, Valkey, gateway, admin)
docker compose -f ../docker-compose.yml up -d --build

# 3. Засей словари тенанта
../seed-tenant.sh

# 4. Открой test-prompt.md — 4 Postman-запроса для всех сценариев
#    (особенно Request 2: словарные данные без PII — проверка маскировки)
#    или запусти автоматический тест:
./test-ollama.sh
```

## Конфигурация

`config.yaml` настраивает:

- **Ollama provider**: `api_type: ollama`, `base_url: http://host.docker.internal:11434`
- **Routing**: модель `gemma3:4b` → Ollama
- **Tenant**: `default` с PII-правилами (email, phone, SSN — block)
- **Shield**: `action_on_suspicious: mask` — словарные значения маскируются перед отправкой к LLM
- **Analytics**: сбор usage (batch каждые 5 секунд)

## Ожидаемое поведение

### Словарная маскировка (действует всегда)

При любом запросе shield находит в тексте совпадения со словарями тенанта (имена, отделы, проекты) и **заменяет** их на `{{dict.<id>.<N>}}` перед отправкой к LLM. В ответе все плейсхолдеры восстанавливаются обратно.

### PII-маскировка

PII-правила (email, phone, SSN) обнаруживаются PII-детектором. Shield **заменяет** обнаруженные PII-фрагменты на `{{pii.<label>.<N>}}` перед отправкой к LLM, а в ответе восстанавливает оригиналы. `action_on_suspicious: mask` — при обнаружении PII запрос не блокируется, PII и словари маскируются.

### Сценарии (4 запроса в `test-prompt.md`)

| Запрос | Словари | PII | Ожидание |
|--------|---------|-----|----------|
| 1. Базовая проверка | нет | нет | 200, clean |
| 2. Словари без PII | **да** | нет | **200, `{{dict.*}}` → unmask** |
| 3. PII + словари | **да** | **да** | **200, `{{dict.*}}` + `{{pii.*}}` → unmask** |
| 4. Streaming | нет | нет | SSE chunks |

## Проверка

```bash
# Проверить что тенант создан и PII-правила активны
curl -s http://localhost:8082/api/v1/tenants/default \
  -H "Authorization: Bearer sk-test-default" | jq .

# Посмотреть usage analytics
curl -s "http://localhost:8082/api/v1/analytics/tokens?period=day" \
  -H "Authorization: Bearer sk-test-default" | jq .

# Посмотреть логи shield
docker logs maskchain-gateway --tail 50
```

## Структура

```
examples/ollama/
├── README.md          # этот файл
├── config.yaml        # конфиг gateway с Ollama
├── test-prompt.md     # 4 Postman-запроса для разных сценариев
└── test-ollama.sh     # автоматический тест маскировки словарей
```
