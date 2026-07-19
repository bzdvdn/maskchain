# Prompt Injection Shield План

## Phase Contract

Inputs: spec (`prompt-injection-shield`), inspect report (pass), repo surfaces (domain detector pattern, entity types, registry, ScanPipeline, config).
Outputs: plan, data model (no-change).
Stop if: — (spec и inspect чисты).

## Цель

Добавить новый тип детектора `prompt_injection` в Content Shield. Это чистое расширение существующей архитектуры детекторов: новый struct, имплементирующий `Detector` interface, новая константа `DetectorType`, набор built-in pattern-ов в коде. Без изменения БД, API-контрактов и UI.

## MVP Slice

- AC-001 (регистрация) + AC-002 (детекция) + AC-003 (нет false positives) + AC-004 (built-in patterns ≥ 20) — базовый детектор, готовый к использованию.

## First Validation Path

```bash
# скопировать существующий интеграционный тест PIIDetector,
# заменить на PromptInjectionDetector с pattern "ignore previous instructions"
go test ./src/internal/domain/shield/detector/ -run TestPromptInjectionDetector -v
```

## Scope

- Новый файл `src/internal/domain/shield/detector/promptinjectiondetector.go` — основная реализация
- Изменение `src/internal/domain/shield/entity/detector_type.go` — новый `DetectorTypePromptInjection`
- Тесты в `promptinjectiondetector_test.go` (AC-001..004), `service_test.go` (AC-005)
- Опциональная конфигурация в `src/internal/infra/config/config.go` (чувствительность/пороги)
- DI-регистрация в gateway binary (cmd/gateway/main.go или app/wiring.go)
- `entity.Detector`, `entity.Pattern`, `ScanPipeline`, `DetectorRegistry` — не меняются (используются as-is)

## Performance Budget

- SC-001: < 1ms на 10KB текста. Ожидание: ~50µs (простой contains/strings.EqualFold по списку из 20-30 паттернов).
- `none` для памяти — overhead минимальный (compile-time строковые литералы).

## Implementation Surfaces

| Surface | Изменение | Причина |
|---------|-----------|---------|
| `src/internal/domain/shield/entity/detector_type.go` | add const `DetectorTypePromptInjection` | Новый тип детектора |
| `src/internal/domain/shield/detector/promptinjectiondetector.go` | **NEW** — struct + `Scan()` + built-in patterns | Основная логика |
| `src/internal/domain/shield/detector/promptinjectiondetector_test.go` | **NEW** — unit тесты AC-001..004 | Покрытие |
| `src/internal/domain/shield/detector/registry_test.go` | add test case для prompt_injection | AC-001 |
| `src/internal/domain/shield/service/service_test.go` | add test case с PromptInjectionDetector | AC-005 |
| `src/internal/infra/config/config.go` | add `PromptInjectionConfig` sub-struct (optional) | Пороги чувствительности |
| `src/cmd/gateway/main.go` / `src/internal/app/wiring.go` | зарегистрировать PromptInjectionDetector в DI | Подключение к пайплайну |

## Bootstrapping Surfaces

- `none` — структура репозитория (detector + entity + config) уже существует.

## Влияние на архитектуру

- Локальное: новый файл в существующем пакете `detector`, новая константа в `entity`.
- Влияние на интеграции: нет — новый детектор подключается через тот же `DetectorRegistry`.
- Migration: нет.

## Acceptance Approach

- AC-001: unit test `registry.Register(entity.DetectorTypePromptInjection, d)` → `registry.Types()` contains `"prompt_injection"`. Surface: `registry_test.go`.
- AC-002: unit test `PromptInjectionDetector.Scan(ctx, text_with_injection)` → result has `DetectorType="prompt_injection"`. Surface: `promptinjectiondetector_test.go`.
- AC-003: unit test `PromptInjectionDetector.Scan(ctx, clean_text)` → empty results. Surface: `promptinjectiondetector_test.go`.
- AC-004: unit test `NewPromptInjectionDetector().BuiltinPatterns() >= 20`. Surface: `promptinjectiondetector_test.go`.
- AC-005: unit test `ScanPipeline.Execute(detectors, injection_text)` → `Status() == ScanStatusBlocked`. Surface: `service_test.go`.
- AC-006: integration test с двумя наборами patterns через `entity.NewDetector()` — проверяет, что tenant-level override работает через существующий механизм `entity.Detector.Patterns()`. Не требует новой инфраструктуры.

## Данные и контракты

- Data model: **no-change** — новый DetectorType enum, без новых таблиц/полей/контрактов. См. `data-model.md`.
- API/event контракты: не меняются. Существующие эндпоинты (`/api/v1/shield/scan`, `/api/v1/shield/mask`) не требуют изменений — новый детектор автоматически участвует через ScanPipeline.
- Config: `ShieldConfig` может получить опциональное поле `PromptInjection *PromptInjectionConfig`, но MVP обходится без него (built-in patterns живут в коде).

## Стратегия реализации

- DEC-001 Built-in patterns в Go-коде
  Why: паттерны prompt injection — это константы (строки), а не пользовательские данные. Хранение их в БД усложняет миграции и не даёт выгоды — tenant-level override уже решается через `entity.Detector.Patterns`. Внешний файл (JSON/YAML) добавил бы зависимость от файловой системы.
  Tradeoff: обновление built-in set требует нового релиза. Для MVP это приемлемо; PostMVP можно добавить auto-update из OWASP feed.
  Affects: `promptinjectiondetector.go`
  Validation: AC-004 (≥ 20 patterns)

- DEC-002 Contains/prefix/suffix матчинг без regex (в MVP)
  Why: regex для 20+ простых фраз даёт ~1µs/pattern vs ~50ns для `strings.Contains`. OWASP injection-фразы — точные строки ("ignore previous instructions", "DAN"), а не паттерны с wildcard. Regex добавит накладные расходы без пользы в MVP.
  Tradeoff: regex понадобится для advanced pattern-ов (например, "you are now [role]"). Это PostMVP.
  Affects: `promptinjectiondetector.go`
  Validation: AC-002, SC-001

- DEC-003 PromptInjectionDetector не зависит от entity.Detector/Pattern — использует собственную built-in структуру
  Why: built-in patterns живут в коде детектора, а tenant-level override работает через передачу `[]entity.Pattern` как внешнего списка. Детектор мержит built-in + tenant patterns на уровне Scan.
  Tradeoff: детектор имеет два источника истины (built-in vs tenant). Нужно чётко документировать, что tenant patterns имеют приоритет при совпадении.
  Affects: `promptinjectiondetector.go`
  Validation: AC-006

## Incremental Delivery

### MVP (Первая ценность)

- Задачи: const DetectorType + struct + Scan() + built-in patterns + регистрация в registry + unit тесты
- AC: AC-001, AC-002, AC-003, AC-004
- Проверка: `go test ./src/internal/domain/shield/detector/ -run TestPromptInjection`

### Итеративное расширение

- Шаг 2: интеграция с ScanPipeline + тест (AC-005)
- Шаг 3: tenant-level override patterns + интеграционный тест (AC-006)
- Шаг 4 (PostMVP): config-секция для порогов/чувствительности

## Порядок реализации

1. `entity/detector_type.go` — константа (1 строка, блокера нет)
2. `promptinjectiondetector.go` — struct + built-in patterns + Scan()
3. `promptinjectiondetector_test.go` — AC-001..004
4. `registry_test.go` — add prompt_injection case (AC-001)
5. `service_test.go` — ScanPipeline integration (AC-005)
6. DI wiring — регистрация в gateway (AC-005, AC-006)
7. Tenant override тест (AC-006) — после DI wiring для полного цикла

Шаги 1-4 можно в одном PR. Шаги 5-6 — второй PR. Шаг 7 — третий PR или в составе второго.

## Риски

- Риск 1: built-in patterns быстро устаревают (новые jailbreak-техники появляются еженедельно)
  Mitigation: patterns в коде легко патчить; PostMVP — auto-update механизм.
- Риск 2: false positives на легитимных фразах, содержащих "ignore" или "DAN"
  Mitigation: AC-003 требует теста на clean-текст; tenant-level override позволяет отключать/кастомизировать patterns.
- Риск 3: производительность при 1000+ tenant-specific patterns
  Mitigation: MVP — 20-30 built-in patterns, O(n) contains. Если tenant patterns > 100 — перейти на Aho-Corasick (как в dictionary detector). SC-001 гарантирует < 1ms на 10KB.

## Rollout и compatibility

- Специальных rollout-действий не требуется. Новый детектор регистрируется в DI и начинает участвовать в ScanPipeline после деплоя. Tenant override работает через существующий механизм `entity.Detector` — обратная совместимость полная.

## Проверка

- Unit тесты: `promptinjectiondetector_test.go` (AC-001, AC-002, AC-003, AC-004), `service_test.go` (AC-005)
- Integration тест: AC-006 (tenant override через HTTP API)
- DEC-001: AC-004 (≥ 20 patterns в built-in)
- DEC-002: AC-002 + SC-001 (детекция работает без regex, latency < 1ms)
- DEC-003: AC-006 (tenant patterns имеют приоритет)

## Соответствие конституции

- нет конфликтов. Content Shield — core domain (Prompt Injection Shield — новый детектор внутри него). Go + DDD/Clean Architecture. Тенанты в PostgreSQL через существующую модель. Язык: docs=ru, agent=ru, comments=en — соблюдено.
