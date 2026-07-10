# Базовые детекторы Content Shield

## Scope Snapshot

- In scope: Реализация базовых типов детекторов Content Shield: PII (email, phone, SSN, passport), secrets (API key, JWT, private key), financial (Luhn, IBAN, SWIFT), PHI (ICD-10). Включает интерфейс Detector, registry, DetectorResult с позициями и confidence.
- Out of scope: Интеграция с Microsoft Presidio; конфигурация детекторов через профили справочников; pipeline сканирования; actions (block/redact/mask/alert).

## Цель

Разработчик Content Shield получает набор готовых, протестированных детекторов для PII, secrets, финансовых данных и PHI, которые можно регистрировать и вызывать через единый интерфейс. Успех фичи измеряется прохождением unit-тестов с граничными случаями для каждого детектора.

## Основной сценарий

1. Разработчик определяет Detector interface с методом `Scan(ctx, text) []DetectorResult`.
2. DetectorRegistry позволяет зарегистрировать и получить детектор по типу.
3. Каждый конкретный детектор (PII, secrets, financial, PHI) имплементирует интерфейс.
4. При сканировании текста детектор возвращает DetectorResult: тип совпадения, фрагмент, позиции start/end, confidence.
5. Для financial-детектора Luhn-проверка применяется перед выдачей результата.

## User Stories

- P1 Story: Разработчик Shield может вызвать готовый PII-детектор (email, phone, SSN, passport) для произвольного текста и получить структурированный результат со всеми совпадениями.
- P1 Story: Разработчик Shield может вызвать Secrets-детектор (API key, JWT, PEM private key) для произвольного текста и получить структурированный результат со всеми совпадениями.
- P1 Story: Разработчик Shield может вызвать Financial-детектор (Luhn, IBAN, SWIFT) для произвольного текста и получить структурированный результат с позициями и confidence.
- P1 Story: Разработчик Shield может вызвать PHI-детектор (ICD-10 коды) для произвольного текста и получить структурированный результат.
- P2 Story: Разработчик может зарегистрировать кастомный детектор в реестре и вызывать его через единый интерфейс.

## MVP Slice

Реализация 4 типов детекторов + registry + DetectorResult + unit-тесты для граничных случаев. Покрывает AC-001–AC-012.

## First Deployable Outcome

После первого implementation pass можно:
- Скомпилировать пакет `detector` без ошибок
- Запустить `go test ./src/internal/domain/shield/detector/...` и получить зелёные тесты для всех детекторов
- Продемонстрировать корректную работу Luhn-проверки на наборе валидных/невалидных номеров

## Scope

- Пакет `src/internal/domain/shield/detector/` с файлами:
  - `detector.go` — Detector interface, DetectorResult struct
  - `registry.go` — DetectorRegistry
  - `piidetector.go` — email, phone, SSN, passport regexes
  - `secretsdetector.go` — API key, JWT, PEM patterns
  - `financialdetector.go` — Luhn check, IBAN regex, SWIFT regex
  - `phidetector.go` — ICD-10 коды
  - `piidetector_test.go` — unit-тесты PII с граничными случаями
  - `secretsdetector_test.go` — unit-тесты Secrets с граничными случаями
  - `financialdetector_test.go` — unit-тесты Financial с Luhn-проверкой
  - `phidetector_test.go` — unit-тесты PHI
- DetectorResult включает: тип детектора, совпавший фрагмент, start/end позиции, confidence (0.0–1.0)
- Registry позволяет регистрировать детекторы по DetectorType и получать их по типу

## Контекст

- Домен Shield уже содержит entity Detector, Pattern, Incident, ScanResult — новые детекторы будут реализовывать интерфейс сканирования и возвращать результаты, которые могут быть конвертированы в Incident.
- Конституция предписывает Microsoft Presidio для PII detection — данный spec покрывает начальный regex-based слой; Presidio-адаптер является будущим расширением.
- Все детекторы stateless; registry может быть проинициализирован при старте приложения.
- Luhn-алгоритм встроен в financial-детектор для верификации номеров карт.
- Маскинг должен быть обратимым (template-based replacement, а не глухая redact): найденный детектором фрагмент и его позиция используются downstream для замены на template-ссылку, которая позже резолвится через `/unmask`. Fragment в `DetectorResult` — это данные, подлежащие шаблонной замене и восстановлению, а не просто лог.

## Зависимости

- Зависит от entity.DetectorType (из `domains/shield/entity/detector_type.go`) и типов из пакетов `entity` и `value`.
- Внешних библиотек не требуется: детекторы используют стандартную библиотеку Go (regexp, strings, strconv).
- `none` меж-спековых зависимостей (downstream: маппинг mask_id → фрагменты будет храниться в PG + Valkey — отдельный spec).

## Требования

- RQ-001 Система ДОЛЖНА предоставлять Detector interface с методом `Scan(text string) ([]DetectorResult, error)`
- RQ-002 Система ДОЛЖНА предоставлять DetectorResult с полями: DetectorType, Fragment, StartPos, EndPos, Confidence
- RQ-003 Система ДОЛЖНА предоставлять DetectorRegistry для регистрации и получения детекторов по DetectorType
- RQ-004 PII-детектор ДОЛЖЕН обнаруживать email-адреса, телефонные номера (международный формат), SSN (###-##-####), номера паспортов РФ (XX XXXX XXX)
- RQ-005 Secrets-детектор ДОЛЖЕН обнаруживать API-ключи (по префиксам sk-, pk-), Bearer-токены, JWT (три base64-сегмента), PEM-блоки приватных ключей
- RQ-006 Financial-детектор ДОЛЖЕН обнаруживать номера банковских карт с Luhn-проверкой, IBAN, SWIFT/BIC коды
- RQ-007 PHI-детектор ДОЛЖЕН обнаруживать медицинские ICD-10 коды (буква + две цифры, опционально точка + 1-2 цифры)
- RQ-008 Каждый детектор ДОЛЖЕН иметь unit-тесты, покрывающие граничные случаи: пустой ввод, частичные совпадения, специальные символы, контекстные ложные срабатывания
- RQ-009 Детектор ДОЛЖЕН возвращать пустой слайс (не nil) при отсутствии совпадений
- RQ-010 Confidence ДОЛЖЕН быть в диапазоне [0.0, 1.0]; для точных regex-совпадений — 1.0

## Вне scope

- Интеграция с Microsoft Presidio — вынесено в отдельный spec
- Конфигурация детекторов через профили справочников
- Pipeline сканирования (вызов нескольких детекторов, агрегация)
- Actions (block/redact/mask/alert)
- PHI: коды CPT, LOINC, SNOMED — только ICD-10
- Поддержка нелатинских алфавитов для PII (кириллические форматы)
- Многопоточное и параллельное сканирование

## Критерии приемки

### AC-001 Detector interface определён и регистрируем

- Почему это важно: единый контракт позволяет подключать любые детекторы через общий registry.
- **Given** существующий пакет detector
- **When** разработчик определяет интерфейс Detector с методом `Scan(text string) ([]DetectorResult, error)`
- **Then** все конкретные детекторы имплементируют этот интерфейс
- **Evidence**: компиляция проходит; детекторы могут быть приведены к Detector через assertion

### AC-002 DetectorResult содержит все обязательные поля

- Почему это важно: потребитель должен знать тип, позицию и уверенность совпадения.
- **Given** экземпляр DetectorResult
- **When** читаются его поля
- **Then** доступны DetectorType, Fragment, StartPos, EndPos, Confidence
- **Evidence**: assertion на каждом поле: типы строковые/числовые; Confidence в [0.0, 1.0]; StartPos <= EndPos

### AC-003 PII-детектор находит email, phone, SSN, passport

- Почему это важно: основные PII-категории должны быть перехвачены на уровне входа.
- **Given** текст "Email: test@example.com, Phone: +7 (999) 123-45-67, SSN: 123-45-6789, Passport: 1234 567890"
- **When** вызывается PII-детектор
- **Then** возвращаются 4 DetectorResult с корректными фрагментами и confidence=1.0
- **Evidence**: len(result)==4; Fragment каждого совпадает с оригиналом; Confidence==1.0 для всех

### AC-004 Secrets-детектор находит API key, JWT, PEM private key

- Почему это важно: утечка secrets — наиболее вероятная угроза AI gateway.
- **Given** текст с API ключом "sk-abc123def456", JWT "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNrvP5FQw0QJ0Q0J0Q", PEM-блоком "-----BEGIN PRIVATE KEY-----\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQ==\n-----END PRIVATE KEY-----"
- **When** вызывается Secrets-детектор
- **Then** возвращаются 3 DetectorResult с типами "api_key", "jwt", "private_key"
- **Evidence**: len(result)==3; типы соответствуют ожидаемым

### AC-005 Financial-детектор находит номера карт с Luhn, IBAN, SWIFT

- Почему это важно: финансовые данные защищены PCI DSS и требуют контрольной суммы.
- **Given** текст с номером карты 4532015112830366 (Luhn-valid), IBAN GB82WEST12345698765432, SWIFT CHASUS33
- **When** вызывается Financial-детектор
- **Then** возвращаются 3 DetectorResult
- **Evidence**: len(result)==3; номера карт прошли Luhn; IBAN/SWIFT соответствуют regex

### AC-006 Luhn-невалидные номера карт не детектятся

- Почему это важно: false positives для платёжных номеров недопустимы.
- **Given** текст с номером 4532015112830367 (Luhn-invalid)
- **When** вызывается Financial-детектор
- **Then** номер карты не включается в результат
- **Evidence**: результат не содержит детекций с типом "credit_card"

### AC-007 PHI-детектор находит ICD-10 коды

- Почему это важно: медицинские коды — чувствительные данные для HIPAA.
- **Given** текст с ICD-10 кодами "A00.0, B99.9, J45.0"
- **When** вызывается PHI-детектор
- **Then** возвращаются 3 DetectorResult
- **Evidence**: len(result)==3; Fragment каждого совпадает с ожидаемым кодом

### AC-008 Registry позволяет регистрировать и получать детекторы

- Почему это важно: централизованное управление детекторами для pipeline.
- **Given** пустой DetectorRegistry
- **When** регистрируется PII-детектор под типом "pii"
- **Then** Get("pii") возвращает детектор; Get("unknown") возвращает nil
- **Evidence**: Get известного типа не nil; Get неизвестного — nil

### AC-009 Пустой ввод не вызывает детекций

- Почему это важно: детекторы должны корректно обрабатывать пустой/нулевой ввод.
- **Given** пустая строка ""
- **When** вызывается любой детектор
- **Then** возвращается пустой слайс (не nil)
- **Evidence**: len(result)==0; result != nil

### AC-010 Специальные символы и граничные форматы обрабатываются без паники

- Почему это важно: невалидный ввод не должен ломать детектор.
- **Given** строка со спецсимволами "\x00\n\t\r!@#$%^&*()_+{}[]|\\:;\"'<>,.?/~`"
- **When** вызывается каждый детектор
- **Then** ни один детектор не паникует
- **Evidence**: вызов завершается успешно, результат — пустой слайс

### AC-011 Все точные regex-совпадения имеют confidence=1.0

- Почему это важно: confidence — основа для фильтрации в политиках.
- **Given** точное regex-совпадение в любом детекторе
- **When** детектор возвращает результат
- **Then** Confidence == 1.0
- **Evidence**: assertion на каждом результате

### AC-012 Детекторы возвращают корректные start/end позиции

- Почему это важно: позиции нужны для redaction/mask операций.
- **Given** текст "My email is test@example.com"
- **When** PII-детектор находит email
- **Then** StartPos и EndPos указывают на правильные позиции
- **Evidence**: текст[StartPos:EndPos] == "test@example.com"

## Допущения

- Все детекторы stateless и thread-safe (без shared mutable state)
- Regex-паттерны покрывают основные международные форматы; локализованные форматы — отдельный spec
- Luhn-алгоритм применяется к последовательностям цифр длины 13–19
- PHI-детектор ограничен ICD-10; CPT, LOINC, SNOMED — вне scope
- Confidence=1.0 для точных regex-совпадений; <1.0 только для вероятностных методов (будущие расширения)
- Результаты детекции используются для template-based replacement (обратимый маскинг), а не для необратимой redact. Позиции StartPos/EndPos критичны для точной замены.

## Критерии успеха

- SC-001: `go test ./src/internal/domain/shield/detector/...` проходит без ошибок
- SC-002: Покрытие кода >80% для каждого детектора (`go test -cover`)

## Краевые случаи

- Пустая строка / только пробелы / только спецсимволы
- Частичные совпадения (напр., "test@example" без TLD, "123-45" неполный SSN)
- Внедрённые совпадения (PII внутри JSON/Base64)
- Длинные строки (>10KB)
- Пересекающиеся совпадения (один паттерн содержит другой)
- Номера без Luhn-валидации (игнорируются Financial-детектором)
- Контекстные ложные срабатывания ("Bearer" в обычном тексте без токена)

## Открытые вопросы

- `none`
