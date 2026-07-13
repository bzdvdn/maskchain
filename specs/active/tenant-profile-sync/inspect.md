---
report_type: inspect
slug: tenant-profile-sync
status: pass
docs_language: ru
generated_at: 2026-07-13
---

# Inspect Report: tenant-profile-sync

## Scope

- snapshot: Проверка spec на tenant DB + config sync, profile YAML import, удаление X-Shield-Profile-Slug
- artifacts:
  - CONSTITUTION.md (.speckeep/constitution.summary.md)
  - specs/active/tenant-profile-sync/spec.md

## Verdict

- status: pass

## Errors

- none

## Warnings

- **AC-006**: `Content-Type: application/x-yaml` — это фиксация конкретного media type в spec. Если планируется поддержка multipart/form-data (file upload), стоит сделать выбор на уровне plan/tasks, не в spec.
- **AC-008a**: `HTTP 502 при отсутствии совпадений` — 502 возникает только при отсутствии настроенного LLM-провайдера. Если провайдер есть, статус будет 200. AC стоит дополнить: "502, если провайдера нет; 200, если есть и запрос прошёл". Либо переформулировать как "shield не блокирует (X-Shield-Status: continue), HTTP-статус зависит от провайдера".

## Questions

- **Startup sync + cache warm**: В spec сказано "blocking sync tenant'ов + прогрев кэша". Прогрев кэша — это profile_cache.warm_on_startup. Он уже существует. Но sync tenant'ов — новый. Нужно ли дожидаться sync tenant'ов ДО прогрева кэша или они независимы? Если sync tenant'ов добавляет tenant'ов в БД, а кэш профилей грузится по profile_slug из tenant'ов — sync должен быть первым.
- **Tenant resolver refresh**: После CRUD через admin API resolver должен видеть изменения сразу (runtime). Это implied в AC-005, но механизм (invalidate + reload, или прямой запрос в БД) не специфицирован. Возможно, стоит уточнить на плане.

## Suggestions

- **RQ-004a / AC-008a**: Удаление `X-Shield-Profile-Slug` — стоит проверить test-prompt.md и Postman-коллекции после реализации, т.к. shield scan-запросы больше не используют этот заголовок.
- **AC-009**: "auth с api_key этого tenant возвращает 401" — после delete resolver не должен кешировать удалённый tenant. Если resolver использует in-memory cache, нужен механизм инвалидации (или прямой запрос в БД каждый раз).

## Traceability

- **RQ → AC coverage:**
  - RQ-001 (tenants table) → AC-001
  - RQ-002 (tenant repository) → AC-001, AC-009
  - RQ-003 (tenant admin CRUD) → AC-001, AC-009
  - RQ-004 (tenant resolver) → AC-002, AC-003
  - RQ-004a (remove X-Shield-Profile-Slug) → AC-008a
  - RQ-005 (auth middleware uses resolver) → AC-005
  - RQ-006 (startup sync) → AC-004
  - RQ-007 (profile YAML format) → AC-006
  - RQ-008 (profile import) → AC-006, AC-007
- Все RQ покрыты, все AC имеют Given/When/Then.
- Plan/tasks пока нет.

## Next Step

- safe to continue to plan — замечания в Warnings и Questions стоит учесть на фазе plan/tasks
