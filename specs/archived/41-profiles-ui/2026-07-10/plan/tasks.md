# Profiles UI — Задачи

## Phase Contract

Inputs: plan.md, data-model.md, spec.md.
Outputs: исполнимые задачи с покрытием всех AC.
Stop if: нет.

## Surface Map

| Surface | Tasks |
|---------|-------|
| `ui/` (Vite project root) | T1.1 |
| `ui/src/pages/Profiles/` | T2.2, T2.3, T3.1, T3.2 |
| `ui/src/components/` | T4.1, T4.2 |
| `ui/src/api/` | T1.1 |
| `src/cmd/gateway/main.go` | T1.2 |
| `src/internal/api/dto/profile.go` | T2.1 |
| `src/internal/api/handler/profile/handler.go` | T2.1 |
| `Dockerfile` | T1.3 |
| `Makefile` | T1.3 |
| `ui/vite.config.ts` | T1.1 |
| `ui/src/__tests__/` | T5.2 |
| `src/internal/api/handler/profile/handler_test.go` | T2.1 |

## Implementation Context

- **Цель MVP:** SPA для CRUD профилей, встроенная в gateway. ProfileList с пагинацией + ProfileForm create.
- **Инварианты/семантика:**
  - BrowserRouter + Gin `NoRoute` fallback — чистые URL, fallback после всех API-роутов.
  - Словари — массив `{name, entries[], match_mode}`; препроцессоры — массив `PreprocessorDef{name, type, rules[]}`.
  - API-пагинация: `GET /api/v1/profiles?page=1&page_size=20` → `{data, total, page, page_size}`.
- **Ошибки/коды:** API возвращает `ErrorResponse{error, code, details}`; UI тост/баннер для ошибок.
- **Контракты/протокол:**
  - `GET /api/v1/profiles?page=&page_size=` → `PaginatedResponse{data: ProfileListItem[], total, page, page_size}`
  - `POST /api/v1/profiles` → 201 + `ProfileResponse` (или `ErrorResponse` 400/409/422)
  - `GET /api/v1/profiles/:slug` → 200 + `ProfileResponse` (или 404)
  - `PUT /api/v1/profiles/:slug` → 200 + `ProfileResponse` (или 400/404)
  - `DELETE /api/v1/profiles/:slug` → 204 (или 404)
  - `PATCH /api/v1/profiles/:slug/dictionary` → 200 + `ProfileResponse`
- **Границы scope:**
  - Не делаем: UI логов/инцидентов, dashboard, tags (нет в domain), отдельный image для UI.
  - Backend-изменения минимальны: только PaginatedResponse dto + пагинация в ListProfiles.
- **Proof signals:**
  - `curl localhost:<port>/profiles` → 200 + Content-Type: text/html (AC-001)
  - DevTools XHR `/api/v1/profiles?page=1&page_size=20` → `PaginatedResponse` (AC-002)
  - Форма создания → POST → 201 → редирект (AC-003)
- **References:** DEC-001..DEC-004, DM (PaginatedResponse), RQ-001..RQ-009.

## Фаза 1: Инфраструктура

Цель: Vite-проект, embed-статика, Docker-сборка — foundation, без которого UI не работает.

- [x] T1.1 **Инициализировать Vite + React + TS проект в `ui/`**
  - `npm create vite@latest` с шаблоном `react-ts` в `ui/`
  - Установить `react-router-dom`, настроить BrowserRouter с роутами (`/profiles`, `/profiles/new`, `/profiles/:slug`, `/profiles/:slug/edit`)
  - Создать API-клиент `ui/src/api/profiles.ts` с fetch-функциями для всех эндпоинтов
  - Добавить `ui/.gitignore` (dist, node_modules)
  - Настроить `vite.config.ts` с proxy `/api/*` → `http://localhost:<gateway-port>`
  - **Touches:** `ui/package.json`, `ui/vite.config.ts`, `ui/src/api/profiles.ts`, `ui/src/App.tsx`, `ui/src/main.tsx`, `ui/.gitignore`

- [x] T1.2 **Встроить статику в Go-бинарник (embed + NoRoute)**
  - Добавить `//go:embed ui/dist/*` в `src/cmd/gateway/main.go`
  - Создать `embed.FS` переменную и зарегистрировать `engine.NoRoute(gin.WrapH(http.FileServer(http.FS(staticFS))))` после всех API-роутов
  - Убедиться, что `/health`, `/ready`, `/live`, `/api/*` НЕ перекрываются fallback
  - **Touches:** `src/cmd/gateway/main.go`

- [x] T1.3 **Multi-stage Dockerfile + Makefile таргеты**
  - Расширить `Dockerfile`: stage 1 — `node:20-alpine` для `npm run build`, stage 2 — `golang:1.26-alpine` для Go-сборки, скопировать `ui/dist/` перед `go build`
  - Добавить в `Makefile`: `ui-build` (`npm run build`), `ui-dev` (`npm run dev`)
  - Обновить `docker-build` для зависимости от `ui-build`
  - **Touches:** `Dockerfile`, `Makefile`

## Фаза 2: MVP — список и создание профиля

Цель: минимальная end-to-end ценность — ProfileList с пагинацией + ProfileForm create.

- [x] T2.1 **Backend: PaginatedResponse dto + пагинация ListProfiles**
  - Создать `src/internal/api/dto/pagination.go` с `PaginatedResponse` структурой
  - Модифицировать `ListProfiles` в `handler.go`: парсить `page` (default 1), `page_size` (default 20) из query-параметров, обернуть результат в `PaginatedResponse{data, total, page, page_size}`
  - Обновить handler_test.go: тесты под новый формат ответа
  - **Touches:** `src/internal/api/dto/pagination.go`, `src/internal/api/handler/profile/handler.go`, `src/internal/api/handler/profile/handler_test.go`

- [x] T2.2 **ProfileList страница (таблица, пагинация, loading, empty)**
  - Создать `ui/src/pages/Profiles/ProfileList.tsx`: таблица (slug, name, status), пагинация (page switcher), индикатор загрузки, пустое состояние с CTA
  - Создать `ui/src/pages/Profiles/index.ts` — re-export страниц
  - **Touches:** `ui/src/pages/Profiles/ProfileList.tsx`, `ui/src/pages/Profiles/index.ts`

- [x] T2.3 **ProfileForm create mode с валидацией**
  - Создать `ui/src/pages/Profiles/ProfileForm.tsx`: поля name, slug, description; client-side валидация (slug — только латиница/цифры/дефис, name — required); отображение server-side ошибок (400/409/422)
  - POST на submit → редирект на `/profiles/:slug` при 201
  - Данные формы сохраняются при ошибке (controlled inputs + React state)
  - **Touches:** `ui/src/pages/Profiles/ProfileForm.tsx`

## Фаза 3: Детали, редактирование, удаление

Цель: полный CRUD — просмотр, изменение и удаление профиля.

- [x] T3.1 **ProfileDetail страница**
  - Создать `ui/src/pages/Profiles/ProfileDetail.tsx`: отображение всех полей профиля (name, slug, description, status, dictionaries, preprocessors)
  - Кнопки «Редактировать» и «Удалить»
  - 404 state — сообщение «Профиль не найден» + кнопка «К списку»
  - **Touches:** `ui/src/pages/Profiles/ProfileDetail.tsx`

- [x] T3.2 **ProfileForm edit mode + удаление профиля**
  - ProfileForm: при наличии `:slug` загружать существующий профиль и переключаться в edit mode
  - PUT-запрос на сохранение → редирект на детали
  - Delete: confirm dialog → DELETE → 204 → редирект на `/profiles` + toast-уведомление
  - **Touches:** `ui/src/pages/Profiles/ProfileForm.tsx`, `ui/src/pages/Profiles/ProfileDetail.tsx`

## Фаза 4: Inline-редакторы

Цель: словари и препроцессоры редактируются внутри формы профиля.

- [x] T4.1 **DictionaryEditor (entries key-value manager)**
  - Создать `ui/src/components/DictionaryEditor.tsx`: список entries (key-value), add/remove, match_mode selector (exact/contains/regex/fuzzy)
  - Интегрировать в ProfileForm как раскрывающуюся секцию
  - Entries отправляются в составе `dictionaries[]` при сохранении профиля
  - **Touches:** `ui/src/components/DictionaryEditor.tsx`, `ui/src/pages/Profiles/ProfileForm.tsx`

- [x] T4.2 **PreprocessorEditor (CSV/JSON rule builder)**
  - Создать `ui/src/components/PreprocessorEditor.tsx`: список правил `{name, type, rules[{columns, path, mask}]}`, add/remove/edit
  - Интегрировать в ProfileForm как раскрывающуюся секцию
  - Правила отправляются в составе `preprocessors[]` при сохранении профиля
  - **Touches:** `ui/src/components/PreprocessorEditor.tsx`, `ui/src/pages/Profiles/ProfileForm.tsx`

## Фаза 5: Проверка и edge cases

Цель: automated coverage и обработка граничных случаев.

- [x] T5.1 **Error states (404, 409, network, server validation)**
  - ProfileDetail: 404 — «Профиль не найден» + кнопка «К списку»
  - ProfileForm: 409 (slug conflict) — сообщение «Профиль с таким slug уже существует»
  - ProfileList: network error — toast/баннер «Не удалось загрузить профили. Попробуйте позже.»
  - Все страницы: глобальный ErrorBoundary
  - **Touches:** `ui/src/pages/Profiles/ProfileList.tsx`, `ui/src/pages/Profiles/ProfileDetail.tsx`, `ui/src/pages/Profiles/ProfileForm.tsx`, `ui/src/components/ErrorBoundary.tsx`

- [x] T5.2 **UI-тесты (vitest + React Testing Library)**
  - ProfileList: рендеринг таблицы, пагинация, loading state, empty state
  - ProfileForm: валидация полей, submit с корректными данными, server error display
  - DictionaryEditor: add/remove entry, match_mode selection
  - PreprocessorEditor: add/remove rule
  - API client: mock-ответы, error handling
  - **Touches:** `ui/src/__tests__/ProfileList.test.tsx`, `ui/src/__tests__/ProfileForm.test.tsx`, `ui/src/__tests__/DictionaryEditor.test.tsx`, `ui/src/__tests__/PreprocessorEditor.test.tsx`, `ui/src/__tests__/api.test.ts`

## Покрытие критериев приемки

- AC-001 -> T1.2
- AC-002 -> T2.1, T2.2
- AC-003 -> T2.3
- AC-004 -> T3.2
- AC-005 -> T2.3
- AC-006 -> T4.1
- AC-007 -> T4.2
- AC-008 -> T1.1, T1.3
- AC-009 -> T1.3
- AC-010 -> T3.2
- AC-011 -> T2.2, T5.1

**Все 11 AC покрыты.** Каждый AC имеет ≥1 задачи.
