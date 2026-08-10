# Conversation Logging — API контракты

## Scope

- Связанные `AC-*`: `AC-005`, `AC-006`, `AC-009`
- Связанные `DEC-*`: `DEC-005`
- Новые эндпоинты admin API; существующие контракты не меняются.

## Эндпоинты

### GET /api/v1/conversations — список (только метаданные)

- Auth: admin session (`adminSessionMw`), как у `/api/v1/sessions`/`/api/v1/analytics`.
- Query params:
  - `page` (int, default 1), `per_page` (int, default 20, max 100)
  - `tenant_id` (optional) — фильтр по тенанту
- Ответ `200` (envelope `ApiResponse`, пагинация в корневом поле, как в analytics/sessions):
```json
{
  "data": {
    "items": [
      {
        "id": "uuid",
        "tenant_id": "default",
        "model": "gpt-4o",
        "status": "ok",
        "masked": false,
        "streamed": false,
        "created_at": "2026-08-09T12:00:00Z"
      }
    ]
  },
  "error": null,
  "pagination": { "page": 1, "per_page": 20, "total": 42 }
}
```
- Ответ не содержит содержимого (`request`/`response`/`payload` не выводятся) — контракт AC-006.
- Ошибки: 401 (не авторизован), 500 (внутренняя).

### GET /api/v1/conversations/:id — детали с base64-обёрткой

- Auth: admin session.
- Ответ `200`:
```json
{
  "data": {
    "id": "uuid",
    "tenant_id": "default",
    "model": "gpt-4o",
    "status": "ok",
    "masked": false,
    "streamed": false,
    "created_at": "2026-08-09T12:00:00Z",
    "payload": {
      "request": "<base64 std-encoded Unicode строки запроса>",
      "response": "<base64 std-encoded строки ответа или null>",
      "masking": "<base64 std-encoded JSON-массива [{\"placeholder\":\"[MASK_...]\",\"original\":\"...\"}] или null>"
    }
  }
}
```
- `payload.request` / `payload.response` / `payload.masking` — расшифрованный сервером контент, обёрнутый в **base64 (StdEncoding)**; там, где ответа нет (`blocked`/`error`), `response` = `null`; там, где mask не применялся, `masking` = `null`. Для stream-записей (`streamed=true`) `payload.response` — verbatim SSE-лента (`data: ...\n\n...` + `[DONE]`). Из `payload.masking` фронт получает «как применён mask» (значение → плейсхолдер).
- Фронт декодирует: `TextDecoder` поверх `atob` для корректного UTF-8 (AC-009).
- Ошибки: 401, 404 (id не найден), 500 (ошибка расшифровки/хранилища — ciphertext наружу не уходит).

### Ошибки (общий формат)

- Следуют существующему конверту `ApiResponse` (`dto.NewErrorResponse(code, message)`): `{ "error": { "code": "...", "message": "..." } }` при `skipEnvelope = false` (поведение как в admin API).

## Примечания по безопасности

- Содержимое расшифровывается и отдаётся только на detail-эндпоинте, под admin-аутентификацией.
- List-эндпоинт не дешифрует содержимое (только метаданные из колонок).
- base64 — формат передачи, не граница безопасности; сам контент отдаётся расшифрованным admin-сессией.

## Compatibility

- Новые роуты добавляются в `AdminServer.RegisterConversationHandler` — добавление не ломает существующие эндпоинты.
- Swagger `openapi.yaml` дополняется секцией контрактов выше.