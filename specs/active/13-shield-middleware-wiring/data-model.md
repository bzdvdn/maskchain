# 13 — Content Shield: PII per-tenant + Dict Unmask. Модель данных

## Scope

- Связанные `AC-*`: `AC-001`, `AC-002`, `AC-003`, `AC-004`, `AC-007`
- Связанные `DEC-*`: `DEC-001`, `DEC-002`, `DEC-003`
- Статус: `changed`

## Сущности

### DM-001 Tenant.PIIConfig (новое поле)

- Назначение: PII-правила и поведение сканирования для tenant.
- Источник истины: Tenant entity (загружается tenant-repo).
- Инварианты:
  - `Enabled` — включает/выключает PII-сканирование.
  - `DefaultAction` — `"block"` или `"allow"`; применяется при ошибке engine.Scan.
  - `Rules` — массив правил; если пуст и Enabled=true → engine не вызывается.
- Связанные `AC-*`: AC-001, AC-002, AC-003, AC-004, AC-007
- Связанные `DEC-*`: DEC-001, DEC-003
- Структура:

```go
type PIIConfig struct {
    Enabled       bool      `json:"enabled"`
    DefaultAction string    `json:"default_action"` // "block" | "allow"
    Rules         []PIARule `json:"rules"`
}

type PIARule struct {
    Label   string `json:"label"`   // e.g. "email", "ssn", "phone"
    Type    string `json:"type"`    // "presidio" | "regex"
    Pattern string `json:"pattern"` // regex expression or Presidio entity type
    Action  string `json:"action"`  // "block" | "allow"
}
```

- Жизненный цикл:
  - Создаётся: при загрузке Tenant из tenant-repo (БД/system).
  - Обновляется: через обновление Tenant (sync или API).
  - Zero-value: `PIIConfig{}` → `Enabled=false` → PII не сканируется.

### DM-002 ScanRequest.Rules (изменение)

- Назначение: передача правил сканирования из middleware в ScanUseCase.
- Инварианты:
  - `Rules` заменяет `ProfileSlug`.
  - Если `Rules` пуст — pipeline не строится, возвращается clean.
- Поля:
  - `Text` — `string` (без изменений)
  - **`Rules`** — `[]PIARule` (новое, вместо ProfileSlug)
  - ~~`ProfileSlug`~~ — удалено

### DM-003 ShieldConfig (упрощение)

- Назначение: конфигурация Content Shield middleware.
- Источник истины: YAML-конфиг gateway.
- Изменения:
  - ~~`ProfileMapping map[string]string`~~ — удалено
  - ~~`DefaultAction string`~~ — удалено
  - Остаётся: `ActionOnSuspicious string` (legacy, не меняется)
- Жизненный цикл:
  - Читается при старте gateway.
  - После удаления ProfileMapping/DefaultAction конфиг упрощается.

## Связи

- Tenant.PIIConfig → ScanRequest.Rules → ScanUseCase → Pipeline → DetectorRegistry.
- PIIConfig не зависит от ProfileRepository / Profile entity.

## Производные правила

- `if !tenant.PIIConfig().Enabled || len(tenant.PIIConfig().Rules) == 0` → engine.Scan не вызывается.
- `if engine.Scan err != nil` → `PIIConfig.DefaultAction` (fallback "block").
- `PIARule.Action == "allow"` → детекция логируется, но не блокирует 403.

## Переходы состояний

- Tenant без PIIConfig (nil) → PII disabled.
- Tenant с PIIConfig.Enabled=false → PII disabled.
- Tenant с пустыми Rules → PII disabled (clean).

## Вне scope

- ProfileRepository, ScanPipelineFactory — убираются из middleware-цепочки.
- Presidio pipeline — не меняется.
