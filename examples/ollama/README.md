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

# 4. Запусти авто-тест:
../test-mask.sh

#    Или для другой модели:
../test-mask.sh http://localhost:8080 sk-test-default "mistral-small-latest"
```

## Конфигурация

Единый конфиг в `examples/config.yaml`. Настраивает:

- **Ollama provider**: `api_type: ollama`, `base_url: http://host.docker.internal:11434`
- **Mistral provider**: `api_type: openai`, ключ через `${MISTRAL_KEY}`
- **Routing**: модель `gemma3:4b` → Ollama; `mistral-*` → Mistral
- **Tenant**: `default` с PII-правилами (email, phone, SSN — block)
- **Shield**: `action_on_suspicious: mask` — словарные значения маскируются перед отправкой к LLM
- **Analytics**: сбор usage (batch 5s), cost rates для Ollama ($0) и Mistral

## Структура

```
examples/
├── config.yaml         # единый конфиг (gateway + admin)
├── docker-compose.yml  # стек: PG, Valkey, gateway, admin
├── seed-tenant.sh      # создание тенанта + словари
├── test-mask.sh        # авто-тест shield маскировки
├── test-prompt.md      # Postman-запросы для всех сценариев
└── ollama/             # (устаревшее — конфиг вынесен на уровень выше)
```

## Конфигурация

Общий конфиг в `examples/config.yaml` (теперь единый для gateway и admin). Настраивает:

- **Ollama provider**: `api_type: ollama`, `base_url: http://host.docker.internal:11434`
- **Mistral provider**: `api_type: openai`, ключ через `${MISTRAL_KEY}`
- **Routing**: модель `gemma3:4b` → Ollama; `mistral-*` → Mistral
- **Tenant**: `default` с PII-правилами (email, phone, SSN — block)
- **Shield**: `action_on_suspicious: mask` — словарные значения маскируются перед отправкой к LLM
- **Analytics**: сбор usage (batch 5s), cost rates для Ollama ($0) и Mistral
- **Debug**: admin-token для прямого доступа к API

## Ожидаемое поведение

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
