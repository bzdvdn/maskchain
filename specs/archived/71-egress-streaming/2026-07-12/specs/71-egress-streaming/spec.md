# Egress Streaming: HTTP/HTTPS outgoing client with proxy, SSE, retry

## Scope Snapshot

- In scope: HTTP/HTTPS egress-клиент для outbound вызовов к LLM-провайдерам с поддержкой proxy-диалера, SSE streaming, retry с exponential backoff, cancellation и connection pooling.
- Out of scope: перепроектирование порта `ProviderClient`, балансировка/health-check (это зона routing engine), инициализация клиентов в DI (зона main.go / wire).

## Цель

Routing-слой (FallbackHandler) и будущие provider-адаптеры получают HTTP-клиент для outbound вызовов к LLM API с гарантиями: retry при сбоях, таймаут на провайдер, отмена по контексту (AC-003–AC-007). Пользователь gateway выигрывает: корректная работа за корпоративным outbound proxy, потоковый SSE-ответ без буферизации всего тела, автоматический retry при временных сбоях и контролируемое время ожидания на каждый провайдер. Успех фичи измеряется прохождением интеграционных тестов с реальным HTTP(s) с proxy, SSE-чанк-форвардингом и retry-сценариями.

## Основной сценарий

1. FallbackHandler (или provider-адаптер) вызывает egress-клиент с `ProviderRequest`, контекстом и именем провайдера.
2. Egress-клиент резолвит proxy из окружения (`HTTP_PROXY`/`HTTPS_PROXY`) или конфига, создаёт HTTP-запрос с connection pool (AC-002).
3. Клиент выполняет запрос; если сервер отвечает 5xx или происходит сетевой таймаут — клиент автоматически повторяет запрос с exponential backoff + jitter до исчерпания попыток.
4. Для SSE-запросов (streaming) клиент передаёт чанки построчно через callback/канал, не дожидаясь полного ответа.
5. При отмене контекста все in-flight запросы и retry-цикл немедленно прерываются.
6. Если все попытки исчерпаны — возвращается агрегированная ошибка с указанием причин.

## User Stories

- P1 Story: Оператор gateway настраивает переменные `HTTP_PROXY`/`HTTPS_PROXY` и провайдеры работают за корпоративным proxy без дополнительных действий.
- P2 Story: LLM-запрос со streaming включен — клиент получает токены по мере генерации, без ожидания полного ответа.

## MVP Slice

Минимальный срез, дающий ценность: не-streaming HTTP-вызов через proxy с таймаутом и отменой (AC-001, AC-002, AC-004, AC-005). SSE streaming (AC-003) и retry (AC-006, AC-007) расширяют срез до полной функциональности. Порядок реализации определяется в plan.

## First Deployable Outcome

Gateway запускается с egress-клиентом, настроенным на одного провайдера (OpenAI), проходит HTTP-вызов через локальный squid/socat proxy и возвращает не-streaming ответ. Результат демонстрируется через существующий `ProxyChatCompletion` endpoint.

## Scope

- Реализация `src/internal/adapters/egress/` — HTTP-клиент, proxy dialer, retry-логика, SSE streaming.
- Доработка `ProviderClient` порта для поддержки streaming (новый метод или callback).
- Конфигурация egress: per-provider timeout, max retries, pool size — через существующую `ProviderConfig` или новую секцию `EgressConfig`.
- Чтение системных proxy-переменных (`HTTP_PROXY`, `HTTPS_PROXY`, `NO_PROXY`).
- Graceful cancellation через контекст (прерывание retry-цикла и in-flight запроса).
- Connection pooling: HTTP keep-alive, max idle conns, idle timeout — настраиваемые.
- Интеграционные тесты egress-клиента с локальным HTTP(s)-сервером и proxy.

## Контекст

- Enterprise-сети: обязательна поддержка forward proxy (HTTP/HTTPS).
- LLM API часто используют SSE для streaming ответов — требуется chunk-aware forwarding без буферизации.
- Routing engine уже использует `ports.ProviderClient` — новый адаптер должен быть совместим с существующим `FallbackHandler`.
- `ProviderResponse.Body` — `[]byte`, не поддерживает streaming; spec допускает расширение порта новым методом или callback.
- Retry на уровне egress-клиента НЕ заменяет retry на уровне routing engine — это нижний уровень, куда routing engine может делегировать.

## Зависимости

- Routing engine (`specs/active/70-routing-engine/`) — egress-клиент реализует `ports.ProviderClient`.
- Config bootstrap (`specs/active/01-config-bootstrap/`) — конфигурация egress через viper.

## Требования

- RQ-001 Egress-клиент ДОЛЖЕН поддерживать HTTP/HTTPS proxy через настройку `HTTP_PROXY`/`HTTPS_PROXY`/`NO_PROXY` (переменные окружения и/или конфиг).
- RQ-002 Egress-клиент ДОЛЖЕН поддерживать SSE streaming: чанки передаются вызывающей стороне по мере поступления без буферизации всего тела ответа.
- RQ-003 Egress-клиент ДОЛЖЕН выполнять retry с exponential backoff и jitter при сетевых ошибках для всех методов и для ответов 5xx — только если для провайдера включён опциональный флаг `retry_on_5xx`.
- RQ-004 Egress-клиент ДОЛЖЕН прерывать запрос и retry-цикл при отмене контекста.
- RQ-005 Egress-клиент ДОЛЖЕН поддерживать настраиваемый timeout на провайдера (per-provider).
- RQ-006 Egress-клиент ДОЛЖЕН использовать connection pool с настраиваемыми параметрами (max idle conns, idle timeout, keep-alive).

## Вне scope

- Health-check и балансировка — остаются в routing engine.
- Автоматическое обнаружение proxy (PAC, WPAD) — только явные env vars / конфиг.
- Поддержка SOCKS5 proxy — только HTTP/HTTPS proxy (CONNECT).
- gRPC-клиент — только HTTP/HTTPS (REST) запросы.
- Multiplexing HTTP/2 streams — pooling на уровне HTTP/1.1 keep-alive.
- Метрики egress (latency, retry count) — deferred до слоя observability.

## Критерии приемки

### AC-001 Proxy dialer поддерживает HTTP_PROXY и HTTPS_PROXY

- Почему это важно: Gateway обязан работать в enterprise-сетях, где outbound трафик идёт через корпоративный proxy.
- **Given** egress-клиент сконфигурирован с `HTTP_PROXY=http://proxy.local:3128`
- **When** клиент выполняет HTTP-запрос к `http://api.example.com/v1/chat`
- **Then** запрос проходит через `proxy.local:3128` (CONNECT для HTTPS, прямой для HTTP)
- Evidence: тест с локальным HTTP-прокси-сервером (squid или httptest) подтверждает, что запрос пришёл через proxy.

### AC-002 Connection pooling работает с настраиваемыми параметрами

- Почему это важно: без пула каждое соединение к провайдеру создаётся заново → latency и порт-исчерпание.
- **Given** egress-клиент с `MaxIdleConns=10` и `IdleTimeout=30s`
- **When** клиент выполняет 10 последовательных запросов к одному хосту
- **Then** соединения переиспользуются (проверяется через количество TCP handshake на стороне тестового сервера)
- Evidence: тестовый HTTP-сервер логирует число принятых TCP-соединений — их меньше, чем запросов.

### AC-003 SSE streaming доставляет чанки по мере поступления

- Почему это важно: LLM streaming даёт пользователю первый токен за миллисекунды, а не после генерации всего ответа.
- **Given** egress-клиент вызван с флагом streaming=true
- **When** сервер отправляет SSE-чанки с интервалом 50ms
- **Then** вызывающая сторона получает каждый чанк до завершения полного ответа (не после)
- Evidence: тест с SSE-сервером, отправляющим 10 чанков по 50ms; клиент завершает получение до того, как сервер отправил бы полный response body (~500ms vs >500ms без streaming).

### AC-004 Per-provider timeout прерывает зависший запрос

- Почему это важно: один зависший провайдер не должен блокировать весь gateway.
- **Given** провайдер сконфигурирован с timeout=1s
- **When** запрос к этому провайдеру зависает (сервер не отвечает)
- **Then** клиент возвращает ошибку timeout через ≤1s
- Evidence: тест с сервером, задерживающим ответ на 5s; клиент завершается с `context.DeadlineExceeded` за ≤1.5s.

### AC-005 Cancellation через контекст прерывает запрос и retry

- Почему это важно: при отмене запроса клиентом не должно быть утечек горутин и «висящих» retry-циклов.
- **Given** egress-клиент выполняет запрос с retry (3 попытки, backoff 500ms)
- **When** контекст отменяется на 1-й секунде выполнения
- **Then** клиент немедленно возвращает `context.Canceled` без ожидания retry-пауз
- Evidence: тест с отменой контекста через `tick` — время выполнения меньше суммарного backoff (≤1.5s vs >2s).

### AC-006 Exponential backoff с jitter не даёт «грозы» повторных запросов

- Почему это важно: без jitter при ошибке все клиенты повторят запрос одновременно (thundering herd).
- **Given** egress-клиент с retry=3, base_backoff=100ms
- **When** сервер отвечает 502 на все запросы
- **Then** интервалы между попытками растут экспоненциально (±jitter), и за 10 последовательных вызовов хотя бы один интервал отклоняется от строгого exponential расписания (jitter присутствует)
- Evidence: тест логирует времена попыток — они не детерминированно равны 100/200/400ms.

### AC-007 Retry корректно завершается после исчерпания попыток

- Почему это важно: пользователь должен получить ошибку, а не бесконечный retry.
- **Given** egress-клиент с retry=3
- **When** сервер отвечает 503 на 4 последовательных запроса (1 initial + 3 retry)
- **Then** клиент возвращает ошибку после 3-й retry-попытки, не пытаясь снова
- Evidence: тест подтверждает, что к серверу ушло ровно 4 запроса, и статус ответа — ошибка.

## Допущения

- Go `net/http` Transport с `Proxy` function достаточен для CONNECT-туннелирования HTTPS через HTTP-прокси.
- SSE-сервисы провайдеров используют `text/event-stream` и передают чанки как `data: ...\n\n`.
- Retry на сетевые ошибки применяется для всех методов. Retry на 5xx — только для GET и для POST при включённом `retry_on_5xx` в конфиге провайдера.
- Connection pool параметры по умолчанию разумны для типичных LLM-нагрузок (~10 concurrent requests).

## Критерии успеха

- SC-001 SSE streaming: первый байт доставляется клиенту ≤10ms после получения первого SSE-чанка от провайдера.
- SC-002 Connection reuse: при 50 последовательных запросах к одному провайдеру создаётся ≤3 TCP-соединений (keep-alive работает).
- SC-003 Retry overhead: при успешном ответе с первой попытки overhead retry-логики ≤1ms (проверка, что не было sleep).

## Краевые случаи

- `HTTP_PROXY` + `HTTPS_PROXY` заданы одновременно — HTTPS запросы идут через `HTTPS_PROXY`, HTTP через `HTTP_PROXY`.
- `NO_PROXY` содержит домен — запросы к нему идут напрямую.
- Сервер закрывает соединение до завершения SSE (incomplete chunk) — клиент возвращает ошибку с частичными данными.
- Retry на 5xx, но не на 4xx — клиент НЕ retry-ит 400/401/403/404. Retry на 5xx применим только для GET или для POST с явным `retry_on_5xx=true`.
- Таймаут наступает во время retry-паузы — должен сработать cancellation.
- Все попытки retry исчерпаны — возвращается последняя ошибка с указанием количества попыток.

## Открытые вопросы

- ~~Streaming API: callback vs отдельный метод~~ ✓ решено: `Stream(ctx, req) (<-chan ProviderChunk, error)`
- ~~Retry policy для POST vs GET~~ ✓ решено: 5xx — opt-in per-provider, сетевые ошибки — всегда
- Какой backoff-алгоритм предпочтителен: фиксированные множители (100ms, 200ms, 400ms) или full jitter (`rand.Intn(min*2)`)?
- Стоит ли добавить retry budget (макс. retry в единицу времени) для защиты от каскадных ошибок?
- Нужна ли метрика числа активных in-flight соединений для оперативного мониторинга?
