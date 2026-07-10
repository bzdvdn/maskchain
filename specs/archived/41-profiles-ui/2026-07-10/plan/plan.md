# Profiles UI — План

## Phase Contract

Inputs: spec, inspect (pass), repo surfaces (ui/, Dockerfile, Makefile, api/dto, api/handler/profile).
Outputs: plan.md, data-model.md.
Stop if: нет.

## Цель

React SPA для CRUD профилей, встраиваемая в gateway бинарник. Работа идёт в `ui/` (новый Vite-проект) и точечных изменениях в `src/internal/api/handler/profile/` (пагинация ListProfiles). Dockerfile расширяется двухстадийной сборкой (node → go). Никаких новых Go-зависимостей.

## MVP Slice

Скелетон Vite-проекта + Gin embed-статики + страница списка профилей с пагинацией + форма создания. AC-001, AC-002, AC-003, AC-005, AC-008, AC-009, AC-011.

## First Validation Path

```bash
make ui-dev  # terminal 1: Vite HMR on :5173
go run ./src/cmd/gateway/  # terminal 2: API on :8080
# open http://localhost:5173/profiles — see empty list
# create profile via form — see redirect to detail page
```

Или headless: `npm run build && go build ./src/cmd/gateway/ && curl http://localhost:8080/profiles` → HTML.

## Scope

1. `ui/` — новый Vite + React + TypeScript проект со всеми страницами и компонентами.
2. `src/internal/api/handler/profile/handler.go` — ListProfiles: добавление query-параметров `page`, `page_size` и обёртки `PaginatedResponse`.
3. `src/internal/api/dto/profile.go` — добавление `PaginatedResponse` типа.
4. `Dockerfile` — двухстадийная сборка (node:20-alpine → golang:1.26-alpine → distroless).
5. `Makefile` — таргеты `ui-build`, `ui-dev`, обновление `docker-build`.
6. `src/cmd/gateway/main.go` — добавление `//go:embed ui/dist` и регистрация `NoRoute` handler.

Не меняется:
- Domain entity, repository, use case слои — никаких Go-изменений вне api + cmd.
- `ProfileListItem` — остаётся без `description`/`tags`/`created_at`.
- Tags не реализуются — их нет в domain/entity/DTO, и ни один AC их не покрывает (RQ-002 упоминает, но AC-002 не требует tags).

## Performance Budget

- `docker-build` < 3 мин (SC-002).
- LCP < 2 сек при 50 профилях (SC-001).
- Размер встроенной статики < 1 MB gzipped в бинарнике.
- `none` для server-side p99 — обработка статики не создаёт значимой latency.

## Implementation Surfaces

| Surface | Change | Why |
|---|---|---|
| `ui/` | New Vite project | SPA с нуля |
| `ui/src/pages/Profiles/` | New | ProfileList, ProfileDetail, ProfileForm |
| `ui/src/components/` | New | DetectorConfigurator, ReactionSelector, PatternEditor, DictionaryEditor, PreprocessorEditor |
| `ui/src/api/` | New | API client для `/api/v1/profiles/*` |
| `src/internal/api/dto/profile.go` | Add `PaginatedResponse` | Нужен для пагинированного списка |
| `src/internal/api/handler/profile/handler.go` | Extend ListProfiles | Добавить `page`/`page_size` парсинг и обёртку ответа |
| `src/cmd/gateway/main.go` | Add `//go:embed` + `NoRoute` | Встраивание статики |
| `Dockerfile` | Multi-stage (node → go) | Сборка UI перед Go |
| `Makefile` | Add `ui-build`, `ui-dev` | Удобство разработки |

## Bootstrapping Surfaces

- `ui/` — директория существует (пустая с `.gitkeep`). Первым делом: `npm create vite@latest ui -- --template react-ts`.
- `ui/dist/` — результат сборки, `.gitignore`.
- `src/internal/api/dto/pagination.go` — новый файл для `PaginatedResponse`.

## Влияние на архитектуру

- Локальное: Gin-сервер получает дополнительный handler для статики и fallback. Никакого влияния на API-маршруты.
- Dockerfile: двухстадийная сборка добавляет ~2 мин к билду, но не меняет runtime-образ.
- Прямых compatibility-последствий нет — API-изменение (PaginatedResponse) не сломает production, т.к. фича ещё не в production.

## Acceptance Approach

### AC-001 SPA отдаётся gateway
- Surface: `src/cmd/gateway/main.go` (embed + NoRoute)
- Наблюдение: `curl localhost:<port>/profiles` → 200 + Content-Type: text/html

### AC-002 Список с пагинацией
- Surface: `ui/src/pages/Profiles/ProfileList.tsx` + `handler.go` ListProfiles
- Наблюдение: DevTools XHR `/api/v1/profiles?page=1&page_size=20` → 200, пагинация в UI

### AC-003 Создание профиля
- Surface: `ui/src/pages/Profiles/ProfileForm.tsx` + CreateProfile
- Наблюдение: POST → 201 → редирект на `/profiles/:slug`

### AC-004 Редактирование профиля
- Surface: `ui/src/pages/Profiles/ProfileForm.tsx` (edit mode)
- Наблюдение: PUT → 200 → обновлённые поля на странице деталей

### AC-005 Валидация формы
- Surface: `ui/src/pages/Profiles/ProfileForm.tsx`
- Наблюдение: ошибка + все поля сохранены

### AC-006 Inline-редактор словарей
- Surface: `ui/src/components/DictionaryEditor.tsx`
- Наблюдение: entry добавлена, после сохранения и переоткрытия присутствует

### AC-007 Inline-редактор препроцессоров
- Surface: `ui/src/components/PreprocessorEditor.tsx`
- Наблюдение: правило добавлено, после сохранения и переоткрытия присутствует

### AC-008 Dev-режим Vite HMR
- Surface: Makefile `ui-dev`, Vite proxy config
- Наблюдение: DevTools XHR уходит на :5173 и проксируется на gateway

### AC-009 Docker-сборка с UI
- Surface: Dockerfile multi-stage
- Наблюдение: `docker build` успешен, контейнер отдаёт HTML на `/profiles`

### AC-010 Удаление профиля
- Surface: `ui/src/pages/Profiles/ProfileDetail.tsx` (delete button)
- Наблюдение: DELETE → 204 → редирект на `/profiles` + уведомление

### AC-011 Loading/empty state
- Surface: `ui/src/pages/Profiles/ProfileList.tsx`
- Наблюдение: спиннер во время запроса, сообщение при пустом списке

## Данные и контракты

- AC-001, AC-002, AC-003, AC-004, AC-005, AC-006, AC-007, AC-010 — контракт `/api/v1/profiles/*` уже стабилен (40-profiles-api).
- AC-002 требует расширения контракта: `GET /api/v1/profiles` → `PaginatedResponse{data, total, page, page_size}` вместо `[]ProfileListItem`.
- `data-model.md` создан: описывает `PaginatedResponse` и статус остальной модели (`no-change`).
- PreprocessorDef `{name, type, rules[]}` уже определён в `domain/shield/preprocessor/processor.go` — формат ясен, открытый вопрос spec закрыт.
- DictionaryDTO `{name, entries[], match_mode}` — уже определён в `dto/profile.go`.

## Стратегия реализации

### DEC-001: BrowserRouter + Gin NoRoute fallback
- **Why:** Чистые URL без `/#/`. Gin `NoRoute` регистрируется после всех API-роутов и не мешает.
- **Tradeoff:** Требует аккуратности при добавлении новых API-роутов — fallback не должен их перекрывать. Gin обрабатывает роуты в порядке регистрации, поэтому `NoRoute` в конце безопасен.
- **Affects:** `src/cmd/gateway/main.go`
- **Validation:** `curl /profiles` → HTML, `curl /api/v1/profiles` → JSON

### DEC-002: API pagination через query-параметры
- **Why:** Spec требует `?page=&page_size=`. Backend change минимален: 2 query-параметра, обёртка ответа. Без этого UI не сможет показать пагинацию.
- **Tradeoff:** Меняет формат ответа ListProfiles — несовместим с предыдущим. Но API ещё не в production.
- **Affects:** `src/internal/api/handler/profile/handler.go`, `src/internal/api/dto/`
- **Validation:** `GET /api/v1/profiles?page=1&page_size=20` → `{data: [...], total: N, page: 1, page_size: 20}`

### DEC-003: Inline-редакторы (не отдельные страницы)
- **Why:** Spec требует редактирование словарей и препроцессоров внутри формы профиля, а не на отдельных страницах. Accordion/модальные секции в ProfileForm.
- **Tradeoff:** Форма становится длиннее, но сохраняет контекст. Для большого числа entries потребуется scroll/виртуализация внутри секции.
- **Affects:** `ui/src/pages/Profiles/ProfileForm.tsx`, `ui/src/components/DictionaryEditor.tsx`, `ui/src/components/PreprocessorEditor.tsx`
- **Validation:** AC-006, AC-007

### DEC-004: Vite proxy для dev-режима
- **Why:** Разработчик не собирает статику при каждой правке. Vite dev server на :5173 проксирует `/api/*` на Gin.
- **Tradeoff:** Два процесса. Для отладки API-ошибок нужно смотреть оба лога.
- **Affects:** `ui/vite.config.ts`, Makefile
- **Validation:** AC-008

## Incremental Delivery

### MVP (AC-001, AC-002, AC-003, AC-005, AC-008, AC-009, AC-011)

1. Инициализация Vite-проекта, базовый роутинг, embed+NoRoute в gateway.
2. ProfileList страница: загрузка списка, пагинация, loading/empty state.
3. ProfileForm (create mode): поля name, slug, description, submit.
4. Валидация формы (client-side + server error display).
5. Dockerfile multi-stage, Makefile ui-build/ui-dev.

Проверка: `make docker-build && docker run -p 8080:8080 maskchain/gateway:latest` → открыть `/profiles`, создать профиль.

### Итеративное расширение

- **Шаг 2 (AC-004, AC-010):** ProfileDetail страница + edit mode формы + delete.
- **Шаг 3 (AC-006):** DictionaryEditor inline в форме (accordion, add/remove entries).
- **Шаг 4 (AC-007):** PreprocessorEditor inline в форме (add/edit/remove rules).
- **Шаг 5 (AC-004, AC-001, AC-011):** Edge cases (404, 409, network error).

## Порядок реализации

1. **Скелетон:** Vite init → Go embed → NoRoute → Dockerfile → Makefile (закладывает инфраструктуру, без неё ничего не работает)
2. **API-пагинация:** `PaginatedResponse` dto → ListProfiles pagination (backend change, нужен до UI списка)
3. **ProfileList** (MVP): страница списка с пагинацией, loading, empty
4. **ProfileForm create** (MVP): форма создания, валидация
5. **Docker-сборка + dev-mode** (MVP): сквозная проверка
6. **ProfileDetail + ProfileForm edit + delete** (шаг 2)
7. **DictionaryEditor** (шаг 3)
8. **PreprocessorEditor** (шаг 4)
9. **Edge cases + polish** (шаг 5)

Параллельно: шаги 1 и 2 можно делать независимо (Vite init не зависит от Go-изменений). Шаги 6–9 зависят от завершения 3–4.

## Риски

- **Риск:** Vite dev server CORS/proxy настройка — dev-режим может не заработать с первого раза.
  - **Mitigation:** Прокси настраивается в `vite.config.ts`, Gin CORS middleware уже существует. В плане отведено место на отладку.
- **Риск:** Объём статики > 1 MB — влияет на размер бинарника и время сборки.
  - **Mitigation:** Держать зависимости минимальными; Vite tree-shaking + gzip-compress в бинарнике не нужен (Go http.Server сам не gzip'ит embed). Если > 1 MB — обсудить сжатие на уровне Go middleware.
- **Риск:** Backend pagination меняет формат ответа — может сломать интеграции.
  - **Mitigation:** API ещё не в production, единственный consumer — будущий UI. Безопасно.
- **Риск:** Tags из RQ-002 не реализованы — spec содержит неверифицируемое требование.
  - **Mitigation:** Ни один AC не покрывает tags. План осознанно исключает tags из scope. Если они потребуются — отдельная фича.

## Rollout и compatibility

- Изменение формата ListProfiles (raw array → `PaginatedResponse`) — несовместимое, но API не в production. Специальных rollout-действий не требуется.
- Dockerfile multi-stage — новая сборка, старый образ `maskchain/gateway:latest` пересобирается. Без stateful миграций.
- feature-флаг не нужен — UI статика не влияет на API.

## Проверка

### Automated tests

- **Go:** handler_test.go — обновить ListProfiles тест под PaginatedResponse.
- **UI:** `vitest` для компонентов (рекомендация: React Testing Library):
  - ProfileList — рендеринг таблицы, пагинация, loading, empty
  - ProfileForm — валидация, submit, error display
  - DictionaryEditor — add/remove entries
  - PreprocessorEditor — add/remove rules
  - API client — mock-ответы, error handling

### Manual checks

1. `make docker-build && docker run -p 8080:8080 maskchain/gateway` → открыть `/profiles` → SPA загружается (AC-001, AC-009)
2. Создать профиль → редирект на детали (AC-003)
3. Редактировать профиль → поля обновлены (AC-004)
4. Удалить профиль → редирект + уведомление (AC-010)
5. Открыть `/profiles/non-existent-slug` → 404 страница (краевой случай)
6. `make ui-dev` → открыть `localhost:5173` → HMR работает (AC-008)
7. Словари: добавить entry в форму → сохранить → переоткрыть → entry есть (AC-006)
8. Препроцессоры: добавить правило → сохранить → переоткрыть → правило есть (AC-007)

### AC/DEC coverage

| AC-* | Проверка |
|---|---|
| AC-001 | manual check 1 |
| AC-002 | vitest ProfileList + manual |
| AC-003 | vitest ProfileForm + manual 2 |
| AC-004 | vitest ProfileForm + manual 3 |
| AC-005 | vitest ProfileForm validation |
| AC-006 | vitest DictionaryEditor + manual 7 |
| AC-007 | vitest PreprocessorEditor + manual 8 |
| AC-008 | manual 6 |
| AC-009 | manual 1 |
| AC-010 | manual 4 |
| AC-011 | vitest ProfileList loading/empty |

| DEC-* | Проверка |
|---|---|
| DEC-001 | manual (curl /profiles) |
| DEC-002 | handler_test.go + curl |
| DEC-003 | vitest DictionaryEditor + manual |
| DEC-004 | manual 6 |

## Соответствие конституции

- **Конфликтов нет.** React UI — только для управления профилями (const. п.III, п.VIII). Единый бинарник — соответствует native-only data plane (п.VI). Docker compose для локальной разработки (п.VII). Language policy соблюдена (docs=ru, comments=en).
- **Отложено:** tags (const. п.II — не нарушает, т.к. ни один AC не требует).
