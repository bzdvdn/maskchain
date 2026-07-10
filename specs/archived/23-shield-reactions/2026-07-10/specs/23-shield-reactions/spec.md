# Shield Reactions

## Scope Snapshot

- In scope: механизм исполнения реакций на sensitive data после детекции — block, redact, mask, alert — как заменяемые стратегии, выбираемые на основе результата PolicyEvaluator.
- Out of scope: реализация политик (правил who→what→action), визуальный UI реакций, настройка реакций через API профиля.

## Цель

Разработчик интеграции Content Shield получает стандартный набор реакций на sensitive data (блокировка, редактирование, маскирование с возможностью восстановления, логирование), которые автоматически выбираются на основе severity от PolicyEvaluator. Каждая реакция реализована как isolirоvannый, тестируемый компонент, чтобы их можно было комбинировать, заменять и тестировать независимо.

## Основной сценарий

1. После выполнения ScanPipeline и PolicyEvaluator система получает entity.Reaction.
2. ReactionPipeline сопоставляет Reaction с набором ReactionExecutor.
3. Каждый ReactionExecutor выполняет своё действие: возвращает 403, заменяет PII на `***`, заменяет на `{{ <mask_id> }}` или логирует инцидент.
4. Результат реакции (изменённый контент, HTTP-статус, инцидент) возвращается вызывающему слою.

## User Stories

- P1: Как разработчик, я хочу, чтобы при обнаружении Critical severity запрос блокировался с 403 и описанием причины, чтобы sensitive data не покинула систему.
- P2: Как разработчик, я хочу, чтобы при обнаружении Medium/Low severity PII заменялось на `***`, чтобы данные были безопасны, но запрос проходил.
- P2: Как разработчик, я хочу, чтобы при обнаружении High severity инцидент логировался, а запрос проходил, чтобы я мог анализировать инциденты без блокировки трафика.
- P3: Как разработчик, я хочу, чтобы вместо необратимого редэктирования можно было использовать обратимое маскирование с генерацией MaskEntry, чтобы восстановить оригинал через MaskUseCase.UnmaskText.

## MVP Slice

BlockReaction + RedactReaction + AlertReaction, закрывающие AC-001, AC-002, AC-004. MaskReaction (AC-003) — P3, реализуется после готовности AC-003.

## First Deployable Outcome

Интеграционный тест, в котором ScanResult с Critical incidents → PolicyEvaluator возвращает ReactionBlock → ReactionPipeline вызывает BlockReaction → получает HTTP 403 + описание причины.

## Scope

- Пакет `src/internal/domain/shield/reaction/` с интерфейсом ReactionExecutor и реализациями
- BlockReaction: возвращает 403 Forbidden с payload причины
- RedactReaction: замена найденных фрагментов на маску из `*` пропорционально длине
- MaskReaction: замена на `{{ <UUIDv7> }}` через MaskUseCase.MaskFromResults
- AlertReaction: запись инцидента через IncidentRepository, без блокировки
- ReactionPipeline: интерфейс + реализация маппинга Reaction → ReactionExecutor
- Рефакторинг `MaskUseCase.MaskText` → `MaskFromResults` + миграция API handler
- Удаление `MaskText`
- Изолированные unit-тесты для каждого типа реакции
- Интеграция с существующим entity.Reaction и service.PolicyEvaluator

## Контекст

- В кодовой базе уже существует `entity.Reaction` (allow/block/review/log) и `service.PolicyEvaluator`, возвращающий Reaction на основе severity
- Существует `mask.MaskStorage` и `mask.MaskUseCase` для обратимого маскирования
- Существует `entity.Incident` и `IncidentRepository` для логирования инцидентов
- Все реакции должны быть stateless (кроме MaskReaction, которая создаёт запись в MaskStorage)

## Зависимости

- От PolicyEvaluator (entity.Reaction на вход ReactionPipeline)
- MaskReaction: от MaskUseCase.MaskFromResults (принимает pre-scanned `[]detector.DetectorResult`, не перезапускает детекторы) и UUIDv7 генератора
- AlertReaction: от IncidentRepository (сохранение Incident)
- Ни одна реакция не зависит от других реакций

## Требования

- RQ-001 ReactionPipeline должен принимать `entity.Reaction` от PolicyEvaluator и возвращать результат выполнения соответствующего ReactionExecutor.
- RQ-002 BlockReaction ДОЛЖЕН возвращать ошибку с HTTP-статусом 403 и описанием причины блокировки.
- RQ-003 RedactReaction ДОЛЖЕН заменять каждый переданный фрагмент sensitive data на маску из символов `*`, длина которой равна длине фрагмента.
- RQ-004 MaskReaction ДОЛЖЕН заменять каждый фрагмент на `{{ <UUIDv7> }}` через `MaskUseCase.MaskFromResults(ctx, text, maskID, detectorResults)`, который принимает pre-scanned результаты и не перезапускает детекторы.
- RQ-005 AlertReaction ДОЛЖЕН записывать инцидент в IncidentRepository без изменения контента и без ошибки.
- RQ-006 BlockReaction ДОЛЖЕН использовать domain error `ErrBlockedByPolicy` (или эквивалентный sentinel error из пакета shield/errors) для сигнализации о блокировке.
- RQ-007 Каждый ReactionExecutor ДОЛЖЕН быть тестируем изолированно от других реакций и от PolicyEvaluator.
- RQ-008 ReactionPipeline ДОЛЖЕН быть интерфейсом, чтобы адаптеры могли реализовать свою цепочку (логирование, metric wrapper, profile-специфичный pipeline).

## Вне scope

- Изменение существующего entity.Reaction или service.PolicyEvaluator
- API-эндпоинты и хендлеры, использующие реакции (будут в отдельной фиче интеграции shield pipeline)
- GUI/UI для визуализации или настройки реакций
- Каскадные реакции (одновременное выполнение нескольких реакций на один инцидент) — deferred
- Реакция на основе контент-типа (JSON/XML/plaintext) — deferred
- Метрики и мониторинг выполнения реакций — deferred

## Критерии приемки

### AC-001 BlockReaction возвращает 403 с причиной

- Почему это важно: гарантирует, что критичные sensitive data не покинут систему
- **Given** ScanResult содержит хотя бы один incident с Critical severity
- **When** ReactionPipeline получает ReactionBlock от PolicyEvaluator и вызывает BlockReaction
- **Then** возвращается ошибка с HTTP-статусом 403 и сообщением, содержащим описание причины блокировки
- Evidence: вызов BlockReaction.Execute возвращает (nil, domain error с причиной блокировки), где в сообщении ошибки указан тип детектора/severity; ошибка идентифицируется как блокирующая через `errors.Is(err, ErrBlockedByPolicy)`

### AC-002 RedactReaction заменяет PII на маску из `*` пропорционально длине

- Почему это важно: сохраняет позиционное соответствие и упрощает downstream-обработку (parser'ы не ломаются из-за сдвигов)
- **Given** ScanResult содержит список incident-ов с фрагментами текста и их позициями
- **When** вызывается RedactReaction.Execute с исходным текстом и списком фрагментов
- **Then** каждый фрагмент заменён на строку из символов `*` той же длины; результирующий текст сохраняет исходные позиции всех незатронутых символов
- Evidence: для текста `"email: user@example.com"` при incident с фрагментом `"user@example.com"` (длина 16) результат — `"email: ****************"`; каждый символ фрагмента заменён ровно на один `*`

### AC-003 MaskReaction создаёт MaskEntry и заменяет фрагмент на `{{ <mask_id> }}`

- Почему это важно: позволяет обратимо восстановить оригинал через UnmaskText, сохраняя конфиденциальность в хранилище
- **Given** MaskUseCase.MaskFromResults доступен, ScanResult содержит incidents с фрагментами и их позициями
- **When** вызывается MaskReaction.Execute с исходным текстом и ScanResult
- **Then** MaskReaction конвертирует incidents в `[]detector.DetectorResult`, вызывает `MaskUseCase.MaskFromResults`, и получает изменённый текст с placeholder'ами
- Evidence: для фрагмента `"user@example.com"` результат — `"{{ 0193a1b2-... }}"`, и `MaskStorage.Get(ctx, maskID)` возвращает MaskEntry с Replacements[`"{{ 0193a1b2-... }}"`] = "user@example.com"

### AC-004 AlertReaction логирует инцидент без блокировки

- Почему это важно: даёт observability для incident-ов, не влияя на latency запроса
- **Given** IncidentRepository доступен, ScanResult содержит один или несколько incident-ов с High severity
- **When** вызывается AlertReaction.Execute с ScanResult
- **Then** для каждого incident-а сохранена запись в IncidentRepository, ошибка не возвращена, исходный контент не изменён
- Evidence: вызов AlertReaction.Execute возвращает (originalText, nil); `IncidentRepository.ListByProfile` содержит новые записи

### AC-005 ReactionPipeline выбирает правильный executor по Reaction

- Почему это важно: гарантирует, что маппинг Reaction → ReactionExecutor работает корректно
- **Given** ReactionPipeline с маппингом: ReactionBlock → BlockReaction, ReactionLog → RedactReaction, ReactionReview → AlertReaction
- **When** в pipeline передаётся ReactionBlock
- **Then** вызывается BlockReaction.Execute
- Evidence: для ReactionBlock результат содержит ошибку блокировки (BlockReaction); для ReactionLog — заменённый текст (RedactReaction); для ReactionReview — исходный текст без изменений, и AlertReaction.logIncident вызван для каждого incident-а

## Допущения

- PolicyEvaluator уже возвращает корректный entity.Reaction на основе ScanResult (эта логика не меняется)
- MaskStorage и IncidentRepository уже реализованы и доступны для DI
- Фрагменты в ScanResult не перекрываются (один фрагмент не содержит другой)
- MaskReaction генерирует один mask_id на один фрагмент (один-к-одному)

## Критерии успеха

- SC-001 Каждый ReactionExecutor проходит изолированный unit-тест за <100ms
- SC-002 Pipeline выбора реакции (ReactionPipeline) проходит тест на каждый тип Reaction за <100ms

## Краевые случаи

- Пустой список incident-ов: RedactReaction/MaskReaction/AlertReaction возвращают исходный текст без изменений
- Фрагмент с нулевой длиной (StartPos == EndPos): реакция игнорирует такой фрагмент
- MaskStorage.Save возвращает ошибку: MaskReaction возвращает ошибку, не изменяя текст
- IncidentRepository.Save возвращает ошибку: AlertReaction возвращает ошибку
- nil ScanResult: ReactionPipeline возвращает ошибку ErrNilScanResult
- Перекрывающиеся фрагменты: поведение не определено (согласно допущению)

## Открытые вопросы

- none (все открытые вопросы уточнены в ходе refine)
