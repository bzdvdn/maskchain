# Cleanup Profile Repository — Модель данных

## Scope

- Связанные `AC-*`: `AC-004`, `AC-009`, `AC-010`, `AC-011`, `AC-012`
- Связанные `DEC-*`: `DEC-001`, `DEC-003`
- Статус: `changed`
- Удаляются сущности, поля и таблицы; новые сущности не добавляются.

## Сущности

### DM-001 Profile (удаляется)

- Назначение: доменная модель профиля shield-сканирования
- Статус: **deleted**
- Связанные `AC-*`: `AC-004`
- Связанные `DEC-*`: `DEC-001`
- Действие: удалён весь файл `entity/profile.go`, value objects `profile_id.go`, `profile_slug.go`

### DM-002 Dictionary (изменена)

- **До**: `profileSlug value.ProfileSlug` — поле, ProfileSlug() геттер, NewDictionary принимал profileSlug, FindByProfileSlug в DictionaryRepository
- **После**: поле profileSlug удалено; Dictionary больше не привязан к Profile. DictionaryRepository интерфейс удалён целиком. Тенант-словари живут через TenantRepository.GetDictionaries() (JSONB колонка).
- Источник истины: tenant-контекст (TenantRepository.GetDictionaries)
- Связанные `AC-*`: `AC-004`, `AC-007`
- Связанные `DEC-*`: `DEC-001`, `DEC-002`
- Изменения:
  - Удалено поле `profileSlug`
  - Удалён метод `ProfileSlug()`
  - Изменён `NewDictionary` (удалён параметр profileSlug)
  - Удалён DictionaryRepository (repository.go)
  - Удалён PostgresDictionaryRepo (postgres/dictionary.go)
  - Удалён весь dictionaryrepo/ пакет (DictionaryCache, Valkey, LRU, Warmer, PubSub)

### DM-003 Incident (изменена)

- **До**: `profileSlug string` поле, ProfileSlug() геттер, параметр в NewAuditIncident
- **После**: поле и геттер удалены; NewAuditIncident теряет параметр profileSlug; колонка `profile_slug` в таблице `incidents` не дропается
- Связанные `AC-*`: `AC-012`
- Связанные `DEC-*`: `DEC-003`

### DM-004 MaskEntry (изменена)

- **До**: `profileID *value.ProfileID` поле (опционально), колонка `profile_id TEXT` в таблице
- **После**: поле ProfileID удалено; миграция дропает колонку `profile_id`
- Связанные `AC-*`: `AC-010`, `AC-011`

## Связи

- `Dictionary — Profile`: удалена (Dictionary больше не ссылается на Profile; DictionaryRepository удалён)
- `Incident — Profile`: удалена на уровне entity, колонка `profile_slug` в БД сохранена без FK/reference
- `MaskEntry — Profile`: удалена (поле ProfileID + колонка profile_id дропаются)
- `PostgresDictionaryRepo — PostgresProfileRepo`: удалена (оба удалены)
- `DictionaryCache — ProfileRepository`: удалена (DictionaryCache удалён)

## Производные правила

- нет новых производных правил.

## Переходы состояний

- не затрагиваются.

## Вне scope

- `domain/tenant/tenant.go` — `profileSlug` поле (конфигурационный tenant, не shield Profile)
- `incidents.profile_slug` колонка — не дропается

## No-Change Stub

Не применяется — модель данных меняется (удаление сущностей и полей).
