# Audit Incidents — просмотр и экспорт инцидентов Content Shield

## Scope Snapshot

- **In scope:** API endpoints для чтения/фильтрации/экспорта инцидентов + UI-страницы списка и деталей
- **Out of scope:** создание/управление инцидентами (acknowledge/resolve/assign), уведомления, real-time, dashboard-агрегация

## Цель

Security-операторы и администраторы Content Shield получают возможность просматривать историю срабатываний политик: кто, когда, на каком профиле, какой детектор сработал, какое действие применено. Успех фичи измеряется тем, что оператор находит конкретный инцидент за ≤3 перехода и может выгрузить выборку для SIEM/внешнего аудита.

## Основной сценарий

1. Администратор открывает `/incidents` — система загружает таблицу с пагинацией (по умолчанию первые 20)
2. Администратор фильтрует по severity/tenant/profile_slug — таблица обновляется
3. Администратор кликает строку → переход на `/incidents/:id` с полными данными, redacted prompt и response snippet
4. Администратор нажимает «Экспорт» → выбирает CSV или JSON → система скачивает отфильтрованную выборку
5. **Edge:** нет инцидентов → пустая таблица с сообщением «No incidents found»
6. **Edge:** неверный id → 404; неверный формат экспорта → 400

## User Stories

- P1 Story: Как администратор, я хочу видеть таблицу инцидентов с фильтрами по severity, чтобы быстро оценить обстановку.
- P2 Story: Как SOC-аналитик, я хочу открыть детальную карточку инцидента с redacted prompt/response, чтобы понять контекст срабатывания без раскрытия PII.
- P3 Story: Как администратор, я хочу экспортировать выборку инцидентов в CSV/JSON, чтобы передать данные в SIEM или внешний аудит.

## MVP Slice

Наименьший срез: backend read endpoints + миграция БД + базовая UI-таблица. Обязательные AC: AC-001, AC-002, AC-003, AC-006.

## First Deployable Outcome

Поднята миграция `incidents` (если не существует); два API-эндпоинта (list+filter, get by id) отвечают через curl; UI-страница списка рендерит таблицу с данными (без лишней полировки).

## Scope

- `GET /api/v1/incidents` — список с фильтрацией (severity, tenant, profile_slug) и page/page_size пагинацией
- `GET /api/v1/incidents/:id` — детальная запись (все поля)
- `GET /api/v1/incidents/export?format=csv|json` — экспорт отфильтрованной выборки
- Доменная модель `Incident` — добавить поле `responseSnippet`, привести `rawSnippet` → `promptSnippetRedacted`
- SQL-миграция `002_incidents.sql` (создание/alter таблицы `incidents`)
- `IncidentRepository` — добавить `List(ctx, filter)` с фильтрацией и пагинацией
- Use-case слой: `ListIncidents`, `GetIncident`, `ExportIncidents`
- HTTP handler: `api/handler/incident/`
- UI: `/incidents` (таблица + фильтры) и `/incidents/:id` (детали)
- UI: компонент экспорта (кнопка, выбор формата)
- UI API client: `ui/src/api/incidents.ts`

## Контекст

- Сущность `Incident` и `PostgresIncidentRepo` уже существуют (spec 20, 30, 23) — их нужно расширить, не сломав существующие вызовы (AlertReaction)
- IncidentRepository уже содержит `Save`, `FindByID`, `ListByProfile`, `ListByTenant` — это write path от ShieldEngine; spec 60 добавляет read path
- UI следует шаблону React + TypeScript + React Router, как в `ui/src/pages/Profiles/`
- Пагинация page/page_size (как в profiles) — не cursor-based
- Tenant извлекается из контекста запроса (middleware); пока fallback `default`

## Зависимости

- **spec 23-shield-reactions:** AlertReaction уже пишет инциденты через IncidentRepository.Save — spec 60 не должен сломать этот контракт
- **spec 50-shield-engine:** ShieldEngine генерирует ScanResult → AlertReaction → Save; spec 60 читает то, что spec 50+23 записали

## Требования

- RQ-001 Система ДОЛЖНА хранить инцидент с полями: id (uuid), request_id, timestamp, tenant, profile_slug, detector_type, entry_value, severity, action, prompt_snippet_redacted, response_snippet.
- RQ-002 `GET /api/v1/incidents` ДОЛЖЕН возвращать page/page_size пагинацию и фильтрацию по severity, tenant, profile_slug (каждый опциональный).
- RQ-003 `GET /api/v1/incidents/:id` ДОЛЖЕН возвращать полную запись инцидента (все поля RQ-001).
- RQ-004 `GET /api/v1/incidents/export?format=csv|json` ДОЛЖЕН скачивать отфильтрованные инциденты в указанном формате.
- RQ-005 UI `/incidents` ДОЛЖЕН отображать таблицу с колонками timestamp, severity, tenant, profile_slug, detector_type, action и фильтры по severity/tenant/profile_slug.
- RQ-006 UI `/incidents/:id` ДОЛЖЕН показывать все поля инцидента, включая redacted prompt и response snippet.

## Вне scope

- Создание/редактирование/удаление инцидентов (только read)
- Acknowledge/resolve/assign workflow
- Email/Slack/webhook уведомления
- Realtime-обновления (SSE/WebSocket)
- Retention/архивирование
- Push-интеграция с SIEM (только pull-экспорт)
- Dashboard/агрегированная статистика

## Критерии приемки

### AC-001 Список инцидентов с фильтрацией и пагинацией

- Почему это важно: администратор находит инциденты без полного скана БД
- **Given** в БД есть 15 инцидентов (3 critical, 5 high, 7 medium)
- **When** выполнен `GET /api/v1/incidents?severity=critical&page=1&page_size=2`
- **Then** ответ 200, JSON содержит `data` (2 инцидента с severity=critical) и `total: 3`, `page: 1`, `page_size: 2`
- Evidence: curl возвращает 200, структура paginated response совпадает с `dto.PaginatedResponse`

### AC-002 Детальный просмотр инцидента

- Почему это важно: аналитику нужен полный контекст срабатывания
- **Given** инцидент с id=`abc-123` существует
- **When** выполнен `GET /api/v1/incidents/abc-123`
- **Then** ответ 200, JSON содержит все поля: `request_id`, `timestamp`, `tenant`, `profile_slug`, `detector_type`, `entry_value`, `severity`, `action`, `prompt_snippet_redacted`, `response_snippet`
- Evidence: curl возвращает 200 с ожидаемой структурой

### AC-003 Экспорт в CSV

- Почему это важно: оператор передаёт выборку в SIEM
- **Given** есть 3 high-severity инцидента
- **When** выполнен `GET /api/v1/incidents/export?format=csv&severity=high`
- **Then** ответ 200, Content-Type `text/csv`, тело начинается со строки заголовков и содержит 3 строки данных
- Evidence: curl -v показывает Content-Type: text/csv, тело парсится как валидный CSV

### AC-004 Экспорт в JSON

- Почему это важно: оператор передаёт данные в JSON-ориентированные системы
- **Given** есть инциденты
- **When** выполнен `GET /api/v1/incidents/export?format=json`
- **Then** ответ 200, Content-Type `application/json`, тело — массив инцидентов
- Evidence: curl возвращает 200, тело парсится как JSON-массив

### AC-005 UI таблица инцидентов

- Почему это важно: визуальный просмотр без curl
- **Given** пользователь открыл `/incidents`
- **When** страница загружена и API вернул данные
- **Then** отображается таблица с колонками timestamp, severity, tenant, profile_slug, detector_type, action и фильтры по severity/tenant/profile_slug
- Evidence: в браузере видна таблица с данными и контролы фильтров

### AC-006 Пустое состояние

- Почему это важно: пользователь не видит ошибку при отсутствии данных
- **Given** в БД нет инцидентов
- **When** выполнен `GET /api/v1/incidents`
- **Then** ответ 200, `data: []`, `total: 0`
- Evidence: curl возвращает 200 с пустым массивом

### AC-007 Несуществующий инцидент

- Почему это важно: корректная обработка 404
- **Given** инцидента с id=`nonexistent` нет
- **When** выполнен `GET /api/v1/incidents/nonexistent`
- **Then** ответ 404 с error response
- Evidence: curl возвращает 404 с JSON-телом ошибки

### AC-008 Неверный формат экспорта

- Почему это важно: защита от ошибочных запросов
- **Given** запрос с неизвестным форматом
- **When** выполнен `GET /api/v1/incidents/export?format=xml`
- **Then** ответ 400 с error response
- Evidence: curl возвращает 400

## Допущения

- Инциденты уже создаются ShieldEngine → AlertReaction → `IncidentRepository.Save` (spec 50, 23)
- Tenant извлекается из JWT-контекста (middleware); пока заглушка `tenantIDFromContext` возвращает `default`
- `prompt_snippet_redacted` хранит уже redacted строку (redaction — ответственность слоя создания)
- Поле `response_snippet` опционально (может быть NULL) — модель ответа эндпоинта возвращает `response_snippet: null` при отсутствии
- Имя таблицы в БД: `incidents` (как уже используется в PostgresIncidentRepo)
- Пагинация page/page_size (offset = (page-1) * page_size)

## Критерии успеха

- SC-001 `GET /api/v1/incidents` с фильтром возвращает первые 20 записей за <200ms при 10k строк
- SC-002 Экспорт 1000 записей в CSV занимает <2s

## Краевые случаи

- Пустая БД: `GET /api/v1/incidents` → `data: [], total: 0`
- Несуществующий id: `GET /api/v1/incidents/:id` → 404
- Неверный формат экспорта: `GET /api/v1/incidents/export?format=xml` → 400
- Фильтр без совпадений: пустой результат + `total: 0`
- Все фильтры не указаны: возвращаются все инциденты с пагинацией
- Очень длинные prompt/response snippets: UI обрезает через text-overflow ellipsis, API возвращает полные значения

## Открытые вопросы

1. Миграция: таблица `incidents` уже используется в `PostgresIncidentRepo`, но SQL-миграции для неё нет. Создать новую миграцию `002_incidents.sql` или это сделано вне speckeep? **Решение:** создать `002_incidents.sql`, если таблицы нет (CREATE TABLE IF NOT EXISTS) и добавить/изменить колонки (response_snippet, tenant).
2. Поле `tenant` в таблице `incidents` — его нет в текущей схеме. Tenant можно получить через JOIN с `profiles` (profile_slug → tenant_id). Нужна ли денормализация tenant в `incidents` для быстрой фильтрации? **Решение:** денормализовать `tenant` в таблицу `incidents` для производительности фильтрации без JOIN.
3. `response_snippet` отсутствует в текущей сущности — его нужно добавить. Как это повлияет на AlertReaction (он сейчас создаёт инциденты без response)? **Решение:** поле опционально (указатель), AlertReaction передаёт nil.
