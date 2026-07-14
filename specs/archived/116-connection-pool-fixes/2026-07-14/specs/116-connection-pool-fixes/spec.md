# Connection Pool Fixes: баги egress-клиента, per-provider timeout, TLS, circuit breaker, pool isolation

## Scope Snapshot

- In scope: исправление бага MaxIdleConnsPerHost, per-provider timeout, TLS-конфигурация egress-клиента, простой circuit breaker, выделенный http.Transport на провайдера
- Out of scope: сложная логика circuit breaker (sliding window, half-open probing), connection pooling для non-HTTP протоколов, динамическое изменение конфигурации без перезагрузки

## Цель

Разработчик, интегрирующий LLM-провайдеров, получает стабильное и предсказуемое соединение: исправлен баг пула соединений, каждый провайдер изолирован по таймаутам, TLS-настройкам и пулу соединений, а автоматический circuit breaker предотвращает каскадные таймауты при падении одного провайдера. Успех фичи измеряется отсутствием регрессий в тестах egress и корректной работой per-provider конфигурации в интеграционных тестах.

## Основной сценарий

1. Gateway загружает конфигурацию с несколькими провайдерами, у каждого свои timeout, TLS, max_idle_conns_per_host
2. При создании egress-клиента для провайдера используется выделенный `http.Transport` с его per-provider настройками
3. При N последовательных ошибках вызова провайдера circuit breaker временно исключает провайдера из ротации
4. После таймаута circuit breaker провайдер снова доступен для запросов
5. При падении одного провайдера остальные продолжают работать без влияния на их connection pool

## User Stories

- P1: Оператор настраивает разные таймауты и TLS для разных провайдеров — каждый egress-клиент работает с корректными параметрами
- P2: При временной недоступности провайдера circuit breaker автоматически исключает его, не тратя ресурсы на повторные попытки

## MVP Slice

AC-001 + AC-002 + AC-008 (исправление бага, per-provider timeout, изоляция транспорта) — минимальный срез, дающий demonstrable ценность. AC-003–AC-007 (TLS, circuit breaker) — следующий приоритет.

## First Deployable Outcome

Набор тестов egress (существующих + новых), проходящих без регрессий.

## Scope

- `src/internal/adapters/egress/` — pool.go, client.go, новый файл circuit_breaker.go
- `src/internal/infra/config/config.go` — расширение EgressConfig (TLS), парсинг ProviderConfig.Timeout
- `src/internal/adapters/provider/factory.go` — создание egress.Client с per-provider конфигурацией
- `src/internal/domain/routing/` — опционально: интеграция circuit breaker с HealthStatus

## Контекст

- egress-клиент используется для всех outbound LLM-вызовов; изменения не должны ломать существующие прокси-настройки (proxy.go)
- Существующие тесты egress_test.go должны остаться зелёными
- ProviderConfig.Timeout сейчас хранится как string и не парсится — требуется консистентный парсинг
- TLS-конфигурация отсутствует полностью — все LLM-провайдеры работают через внешние сети, TLS обязателен

## Зависимости

- `none` — внешние библиотеки не требуются; circuit breaker реализуется на стандартной библиотеке (sync + time)

## Требования

- RQ-001 Система ДОЛЖНА корректно применять `MaxIdleConnsPerHost` и `DisableKeepAlives` из конфигурации при создании `http.Transport`
- RQ-002 Система ДОЛЖНА парсить `ProviderConfig.Timeout` в `time.Duration` и пробрасывать его в контекст egress-клиента
- RQ-003 Система ДОЛЖНА поддерживать TLS-конфигурацию: custom CA-сертификат, отключение проверки (insecure_skip_verify), mTLS (client cert + key)
- RQ-004 Система ДОЛЖНА иметь circuit breaker: после N последовательных ошибок вызова провайдера — skip provider на T секунд
- RQ-005 Система ДОЛЖНА создавать выделенный `http.Transport` для каждого провайдера, изолируя connection pool

## Вне scope

- Динамическое обновление TLS-сертификатов без перезагрузки процесса
- Сложные стратегии circuit breaker (sliding window, half-open, success count recovery)
- Per-provider rate limiting (это отдельная фича)
- Поддержка mTLS с горячей перезагрузкой сертификатов

## Критерии приемки

### AC-001 MaxIdleConnsPerHost из конфигурации

- Почему это важно: баг приводит к некорректному поведению пула — per-host лимит равен общему лимиту вместо отдельного значения
- **Given** конфигурация `egress.max_idle_conns_per_host = 5` и `egress.max_idle_conns = 100`
- **When** создаётся `http.Transport` через `newTransport()`
- **Then** `transport.MaxIdleConnsPerHost == 5`, а не `transport.MaxIdleConns`
- Evidence: assertion в unit-тесте pool.go; существующий тест `TestConnectionReuse` проходит

### AC-002 Per-provider timeout прокидывается в egress-контекст

- Почему это важно: разные провайдеры требуют разных таймаутов; сейчас таймаут задаётся только на уровне вызывающей стороны
- **Given** `ProviderConfig.Timeout = "30s"` для провайдера "openai"
- **When** фабрика создаёт egress.Client для этого провайдера и вызывается `client.Call(ctx, ...)` без таймаута в переданном ctx
- **Then** egress.Client самостоятельно устанавливает timeout 30s на внутренний контекст запроса
- Evidence: тест `TestPerProviderTimeout` подтверждает, что HTTP-клиент завершает запрос по истечении 30s без внешнего cancel

### AC-003 Custom CA-сертификат

- Почему это важно: enterprise-среды требуют кастомных CA для внутренних провайдеров
- **Given** `egress.tls.ca_cert` указывает на файл PEM с кастомным CA
- **When** создаётся `http.Transport`
- **Then** `transport.TLSClientConfig.RootCAs` содержит загруженный CA
- Evidence: unit-тест с самоподписанным сертификатом, подписанным кастомным CA

### AC-004 InsecureSkipVerify (для внутренних провайдеров)

- Почему это важно: внутренние сервисы могут использовать самоподписанные сертификаты
- **Given** `egress.tls.insecure_skip_verify = true`
- **When** создаётся `http.Transport`
- **Then** `transport.TLSClientConfig.InsecureSkipVerify == true`
- Evidence: assertion после создания транспорта

### AC-005 mTLS

- Почему это важно: некоторые внутренние провайдеры требуют взаимной TLS-аутентификации
- **Given** `egress.tls.cert` и `egress.tls.key` указывают на файлы PEM
- **When** создаётся `http.Transport`
- **Then** `transport.TLSClientConfig.Certificates` содержит загруженную клиентскую пару
- Evidence: unit-тест с тестовыми сертификатами; ручная проверка через tls-диагностику

### AC-006 Circuit breaker открывается после N последовательных ошибок

- Почему это важно: автоматически предотвращает трату ресурсов на заведомо падающий провайдер
- **Given** circuit breaker настроен на 3 ошибки и 10s cooldown
- **When** 3 последовательных вызова к провайдеру завершаются ошибкой
- **Then** 4-й вызов возвращает ошибку "provider skipped by circuit breaker" без реального HTTP-вызова
- Evidence: unit-тест circuit breaker

### AC-007 Circuit breaker восстанавливается после cooldown

- Почему это важно: после временного сбоя провайдер должен автоматически возвращаться в ротацию
- **Given** circuit breaker в открытом состоянии (после N ошибок)
- **When** проходит время cooldown и приходит следующий запрос
- **Then** запрос проходит к провайдеру (реальный HTTP-вызов)
- Evidence: unit-тест с mock времени

### AC-008 Per-provider изоляция connection pool

- Почему это важно: "шумный сосед" не должен истощать пул соединений других провайдеров
- **Given** два провайдера с разными настройками (разные base_url)
- **When** оба провайдера одновременно делают запросы
- **Then** каждый использует свой экземпляр `http.Transport` с независимыми счетчиками соединений
- Evidence: тест проверяет, что `*http.Transport` для разных провайдеров — разные указатели; значения MaxIdleConnsPerHost не пересекаются

## Допущения

- Circuit breaker сбрасывается только по cooldown (без half-open probing) — приемлемо для первой итерации
- TLS-сертификаты загружаются при старте и не перезагружаются без рестарта процесса
- ProviderConfig.Timeout парсится как duration string (совместимо с стандартным `time.ParseDuration`)
- Если egress.TLS не задан — используется стандартный TLS из Go (пул системных CA)

## Критерии успеха

- SC-001 Все существующие тесты egress проходят без изменений (regression-free)
- SC-002 Новые тесты покрывают AC-001 — AC-008

## Краевые случаи

- ProviderConfig.Timeout пустой или невалидный — использовать default-таймаут из EgressConfig
- TLS-файлы не найдены или невалидны — ошибка при инициализации (fail-fast)
- Circuit breaker счётчик переполнения при очень частых ошибках — atomic-счётчик не должен переполняться
- Все провайдеры в circuit breaker — fallback до последнего живого (или ошибка, если все недоступны)
- DisableKeepAlives = true — пул не накапливает idle-соединения (проверить transport.IdleConnTimeout)

## Открытые вопросы

- Должен ли circuit breaker сбрасывать счётчик после успешного вызова в частично открытом состоянии? (решение: нет, только полный cooldown для первой версии)
- Нужен ли отдельный конфиг для circuit breaker (per-provider или глобальный)? (решение: глобальный в EgressConfig для старта)
