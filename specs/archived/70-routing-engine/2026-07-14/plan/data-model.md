---
status: changes
slug: 70-routing-engine
---

# Data Model: 70-routing-engine

## Entities

### Provider

| Field | Type | Description |
|---|---|---|
| Name | string | Уникальное имя провайдера (ключ) |
| BaseURL | string | Базовый URL API провайдера |
| HealthEndpoint | string | URL для health check (опционально) |
| Timeout | time.Duration | Per-provider timeout на запрос |
| Priority | int | Приоритет (не используется в MVP, задел) |
| HealthStatus | HealthStatus | Текущий статус здоровья |

### Route

| Field | Type | Description |
|---|---|---|
| Model | string | Имя модели (ключ для сопоставления) |
| Providers | []string | Упорядоченный список имён провайдеров |

### RoutingRule

| Field | Type | Description |
|---|---|---|
| TenantID | string | Tenant ID ("default" для общих правил) |
| Routes | []Route | Список маршрутов для tenant'а |

## Enums

### HealthStatus

- `Unknown` — статус не проверялся
- `Healthy`
- `Unhealthy`

## Config (YAML)

```yaml
routing:
  providers:
    - name: openai
      base_url: https://api.openai.com
      health_endpoint: /health
      timeout: 30s
      priority: 1
    - name: azure-openai
      base_url: https://my-openai.openai.azure.com
      health_endpoint: /health
      timeout: 30s
      priority: 2
  rules:
    - tenant: default
      routes:
        - model: gpt-4
          providers: [openai, azure-openai]
        - model: gpt-3.5-turbo
          providers: [openai]
```

## Изменения в существующих моделях

- `Config` (infra/config): добавляется поле `Routing *RoutingConfig` с тегами mapstructure/yaml.
- `ShieldConfig.TenantModelMapping` остаётся нетронутым (deprecated, удалять не в этой фиче).
- Никаких изменений в БД или domain/shield.
