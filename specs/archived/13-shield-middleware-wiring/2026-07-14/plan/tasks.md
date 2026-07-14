# 13 — Content Shield: PII per-tenant + Dict Unmask. Задачи

## Phase Contract

Inputs: plan.md, data-model.md, spec.md, repo surfaces (tenant.go, shield.go, scan_usecase.go, config.go, main.go, pipeline_factory.go).
Outputs: задачи с Touches и покрытием AC.
Stop if: — нет.

## Surface Map

| Surface | Tasks |
|---------|-------|
| `src/internal/domain/shield/entity/tenant.go` | T1.1 |
| `src/internal/app/usecase/shield/types.go` | T1.2 |
| `src/internal/app/usecase/shield/scan_usecase.go` | T1.3 |
| `src/internal/app/usecase/shield/pipeline_factory.go` | T1.3 |
| `src/internal/api/middleware/shield.go` | T2.1, T2.2, T2.3 |
| `src/internal/infra/config/config.go` | T2.4 |
| `src/cmd/gateway/main.go` | T2.5 |
| `src/internal/api/middleware/shield_test.go` | T2.6, T3.1, T4.1 |

## Implementation Context

- **Цель MVP:** Tenant.PIIConfig + Rules → engine.Scan → 403 + dict unmask non-streaming
- **Инварианты/семантика:**
  - Tenant владеет PII-правилами и словарями. Middleware читает всё из TenantFromContext.
  - ScanRequest.Rules заменяет ProfileSlug — pipeline строится напрямую из правил.
  - Dict unmask не зависит от PII — работает всегда при наличии маппинга.
  - PIIConfig.DefaultAction применяется только при ошибке engine.Scan.
  - Если PIIConfig.Enabled=false или Rules пуст — engine.Scan не вызывается.
- **Ошибки/коды:**
  - engine.Scan error → лог + PIIConfig.DefaultAction (не паника).
  - Нет правил → PII не сканируется (clean), dict работает.
  - unmask: placeholder не в маппинге → остаётся как есть (no-op).
- **Границы scope:** ScanUseCase меняется (убрать FindBySlug). Provider_handler.go не трогается.
- **Proof signals:** Tenant с правилами → 403; dict-запрос → unmask.
- **References:** DEC-001–DEC-005, DM-001, DM-002, DM-003.

## Фаза 1: PIIConfig + engine

Цель: tenant хранит правила, engine принимает их напрямую.

- [x] T1.1 **Добавить PIIConfig в Tenant entity**
  - Добавить структуры `PIIConfig` и `PIARule` в `entity/tenant.go`
  - Добавить геттер `PIIConfig()` и опцию `WithPIIConfig()`
  - Zero-value: `Enabled=false`, `DefaultAction=""`, `Rules=nil`
  - Touches: `src/internal/domain/shield/entity/tenant.go`

- [x] T1.2 **Заменить ProfileSlug на Rules в ScanRequest**
  - Удалить поле `ProfileSlug string`
  - Добавить поле `Rules []PIARule` (или переиспользовать тип из entity)
  - Touches: `src/internal/app/usecase/shield/types.go`

- [x] T1.3 **ScanUseCase строит pipeline из Rules, а не из ProfileSlug**
  - Убрать `profileRepo.FindBySlug` — больше не нужен
  - Убрать `NewProfileSlug` валидацию
  - Если `len(req.Rules) == 0` → вернуть clean
  - Добавить метод `ScanPipelineFactory.BuildFromRules(ctx, rules []entity.PIARule)`:
    - Для каждого rule: lookup детектора в registry по `rule.Type`
    - Создать DetectorBinding с label, severity (medium), action
  - Touches: `src/internal/app/usecase/shield/scan_usecase.go`, `src/internal/app/usecase/shield/pipeline_factory.go`

## Фаза 2: Middleware + cleanup

Цель: middleware читает PIIConfig, конфиг и main.go упрощаются.

- [x] T2.1 **Middleware читает PIIConfig из Tenant**
  - После `TenantFromContext(c)` — прочитать `tenant.PIIConfig()`
  - Если `!enabled` или `len(Rules) == 0` → не вызывать engine.Scan, продолжить с dict-only
  - Если `enabled && len(Rules) > 0` → `engine.Scan({Text, Rules})`
  - Graceful degradation: при ошибке engine.Scan → лог + `DefaultAction`
  - В лог добавить поля `pii_enabled` (bool), `rules_count` (int)
  - Touches: `src/internal/api/middleware/shield.go`

- [x] T2.2 **Удалить ProfileMapping и DefaultAction из ShieldConfig**
  - Удалить поля из структуры
  - Обновить mapstructure tags (оставить только ActionOnSuspicious)
  - Touches: `src/internal/infra/config/config.go`

- [x] T2.3 **Упростить main.go — убрать ProfileRepository DI**
  - Убрать создание PostgresProfileRepo, DictionaryCache, ScanPipelineFactory, PolicyEvaluator, DefaultReactionPipeline
  - ScanUseCase больше не создаётся в main.go
  - Оставить только `ShieldMiddleware(engine, cfg.Shield, logger)` — engine может быть nil до Presidio
  - Touches: `src/cmd/gateway/main.go`

## Фаза 3: Тесты

Цель: покрыть PII per-tenant и dict unmask.

- [x] T3.1 **Переписать PII-тесты под tenant.PIIConfig**
  - TestPIIConfig_BlocksEmail: tenant с правилом email:block → PII-промпт с email → 403
  - TestPIIConfig_AllowsEmail: tenant с правилом email:allow → тот же промпт → 200
  - TestPIIConfig_Disabled: tenant с Enabled=false → engine не вызван, dict работает
  - TestPIIConfig_EmptyRules: tenant с Enabled=true, Rules=[] → engine не вызван
  - TestPIIConfig_GracefulDegradation: engine error → default_action
  - Удалить тесты ProfileMapping (BlocksPII, NoMappingBlock)
  - Touches: `src/internal/api/middleware/shield_test.go`

- [x] T3.2 **Адаптировать существующие dict unmask тесты**
  - TestShieldProfileMapping_DictUnmask → переименовать в TestDictUnmask (убрать ProfileMapping из названия)
  - Убедиться, что tenant передаётся с словарями через контекст (как сейчас)
  - Streaming тест уже использует tenant с словарями — оставить
  - Touches: `src/internal/api/middleware/shield_test.go`

## Фаза 4: Проверка

- [x] T4.1 **Edge cases**
  - Tenant без PIIConfig (nil) → Enabled=false → PII не сканируется
  - LLM ответ без placeholders → unmask no-op
  - Placeholder с некорректным индексом → не заменяется
  - Touches: `src/internal/api/middleware/shield_test.go`

- [x] T4.2 **Финальная проверка**
  - Проверить покрытие: каждый AC покрыт ≥ 1 задачей
  - go vet, build, all tests pass
  - provider_handler.go не изменён
  - Отметить все задачи как выполненные
  - Touches: `src/internal/api/middleware/shield.go`, `src/cmd/gateway/main.go`

## Покрытие критериев приемки

- AC-001 -> T1.1, T1.2, T1.3, T2.1, T3.1 (PIIConfig + Rules + middleware + test)
- AC-002 -> T1.1, T2.1, T3.1 (per-pattern action allow)
- AC-003 -> T1.1, T2.1, T3.1 (Enabled=false guard)
- AC-004 -> T2.1, T3.1 (graceful degradation)
- AC-005 -> T2.1, T2.2, T2.3, T3.2 (dict unmask non-streaming)
- AC-006 -> T2.1, T3.2 (streaming unmask)
- AC-007 -> T1.1, T2.1, T3.1 (empty rules → no engine call)

## Заметки

- T1.1 → T1.2 → T1.3 — sequential
- T2.1, T2.2, T2.3 — после T1.3, между собой независимы
- T3.1, T3.2 — после T2.1, независимы
- T4.1, T4.2 — последние
- Старые @sk-task маркеры (51-shield-gateway-integration, tenant-profile-sync) не трогать — они относятся к другому скоупу
