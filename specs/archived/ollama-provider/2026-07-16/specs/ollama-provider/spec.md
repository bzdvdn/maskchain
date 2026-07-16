# Ollama Provider

## Scope Snapshot

- In scope: поддержка Ollama как провайдера LLM в gateway — конфигурация, адаптер, роутинг.
- Out of scope: нативная поддержка Ollama API (raw `/api/generate`, `/api/chat`); авто-скачивание моделей; UI для управления Ollama.

## Цель

Разработчик получает возможность поднять локальный Ollama с любой opensource-моделью и тестировать MaskChain (shield, роутинг, analytics) без доступа к OpenAI/Anthropic и без риска утечки API-ключей. Успех: конфиг с `api_type: ollama` работает через тот же proxy pipeline.

## Основной сценарий

1. Разработчик устанавливает и запускает `ollama serve` локально с моделью (например, `llama3.2`).
2. В YAML-конфиге gateway добавляет блок провайдера с `api_type: ollama`, указывает `base_url: http://localhost:11434` и модель в роутинге.
3. Gateway стартует, фабрика провайдеров создаёт OllamaClient, роутинг включает его в список доступных провайдеров.
4. POST /api/v1/chat/completions с моделью, привязанной к Ollama, проксируется на `http://localhost:11434/v1/chat/completions`.
5. При недоступности Ollama gateway возвращает 503 с диагностикой.

## User Stories

- P1 (MVP): разработчик конфигурирует Ollama и получает ответы от локальной модели через стандартный proxy pipeline.
- P2: поддержка raw Ollama API (`/api/generate`) для прямого доступа без OpenAI-совместимости.

## MVP Slice

Адаптер поверх OpenAI-совместимого REST API Ollama (`/v1/chat/completions`) + регистрация в фабрике. AC-001–AC-005.

## First Deployable Outcome

После первого implementation pass: `curl -X POST http://localhost:8080/api/v1/chat/completions -d '{"model":"llama3.2","messages":[{"role":"user","content":"hi"}]}'` возвращает ответ от локальной Ollama.

## Scope

- Адаптер `OllamaClient` в `src/internal/adapters/provider/ollama.go`
- Регистрация `case "ollama":` в фабрике `factory.go`
- Пропуск API-ключа для Ollama (auth headers не отправляются)
- Валидация конфига: `api_type: ollama` не требует `api_keys`
- Обработка ошибок: таймаут, connection refused, 4xx/5xx от Ollama
- Интеграционный тест с real HTTP-сервером (mock Ollama)

## Контекст

- Ollama v0.1.x+ предоставляет OpenAI-совместимый endpoint `/v1/chat/completions` — того же формата, что использует существующий `OpenAIClient`
- Gateway уже работает в enterprise-сетях с outbound proxy — это не влияет на локальный Ollama
- Content Shield, analytics, тенанты не зависят от типа провайдера — всё идёт через `ProviderClient` interface

## Зависимости

- Внешних зависимостей нет: Ollama — отдельный процесс, HTTP-клиент через существующий egress-слой (`egress.Client`)
- Меж-спековых зависимостей нет

## Требования

- RQ-001 Gateway ДОЛЖЕН принимать `api_type: ollama` в конфигурации провайдера и создавать OllamaClient через фабрику
- RQ-002 OllamaClient ДОЛЖЕН имплементировать `ports.ProviderClient` (Call + Stream) через OpenAI-совместимый REST API
- RQ-003 OllamaClient НЕ ДОЛЖЕН отправлять заголовки авторизации (API-ключ опционален или отсутствует)
- RQ-004 При недоступности Ollama gateway ДОЛЖЕН возвращать 503 NO_HEALTHY_PROVIDER с диагностикой
- RQ-005 Валидация конфига ДОЛЖНА разрешать `api_type: ollama` без обязательного `api_keys`

## Вне scope

- Поддержка моделей через Docker Compose / авто-установка Ollama
- Raw API `/api/generate` или `/api/chat` (не OpenAI-совместимые)
- Pull моделей при старте gateway
- Health check endpoint для Ollama (используется базовый `/`)
- UI для выбора/управления Ollama-моделями
- Streaming без SSE (Ollama-native chunked JSON)

## Критерии приемки

### AC-001 Конфигурация Ollama провайдера

- Почему это важно: разработчик должен иметь возможность описать Ollama в YAML без подделки api_keys
- **Given** конфигурационный файл с блоком провайдера `api_type: ollama`, `base_url: http://localhost:11434`, без `api_keys`
- **When** gateway загружает конфиг и проходит валидацию
- **Then** валидация успешна, фабрика создаёт экземпляр OllamaClient, ошибок нет
- Evidence: unit-тест `TestNewOllamaClient_ValidConfig` и отсутствие ошибки валидации при `api_keys: []`

### AC-002 Non-streaming запрос через Ollama

- Почему это важно: базовый use-case — получить ответ от локальной модели
- **Given** запущенный gateway с Ollama провайдером и mock-сервер, отвечающий как Ollama
- **When** клиент отправляет POST `/api/v1/chat/completions` с `{"model":"llama3.2","messages":[{"role":"user","content":"hi"}],"stream":false}`
- **Then** gateway проксирует запрос на mock, получает ответ и возвращает его клиенту с 200
- Evidence: интеграционный тест `TestOllamaClient_Call` с `httptest.NewServer`

### AC-003 Streaming запрос через Ollama

- Почему это важно: streaming — частая потребность для LLM UX
- **Given** запущенный gateway с Ollama провайдером и mock-сервер SSE
- **When** клиент отправляет POST `/api/v1/chat/completions` с `{"stream":true}`
- **Then** gateway проксирует SSE chunks от Ollama клиенту
- Evidence: интеграционный тест `TestOllamaClient_Stream` с `httptest.NewServer` и проверкой полученных chunks

### AC-004 Отсутствие auth-заголовков

- Почему это важно: Ollama не требует авторизации, лишние заголовки могут вызвать проблемы
- **Given** OllamaClient с пустым api_key
- **When** клиент делает Call или Stream
- **Then** исходящий HTTP-запрос не содержит `Authorization` и `X-API-Key` заголовков
- Evidence: тест перехватывает запрос mock-сервером и проверяет отсутствие auth-заголовков

### AC-005 Ошибка при недоступном Ollama

- Почему это важно: разработчик должен понимать, что Ollama не запущен
- **Given** gateway с провайдером, направленным на несуществующий хост
- **When** клиент отправляет запрос
- **Then** gateway возвращает 503 NO_HEALTHY_PROVIDER, лог содержит connection refused/timed out
- Evidence: тест с закрытым портом проверяет статус 503 и ошибку в логе/ответе

### AC-006 Интеграция с routing pipeline (сценарий)

- Почему это важно: Ollama должен работать в полном proxy pipeline с shield и analytics
- **Given** gateway с shield и analytics middleware + Ollama провайдер + локальный `ollama serve`
- **When** отправляется запрос с моделью, маршрутизируемой на Ollama
- **Then** запрос проходит shield (если включён), analytics middleware фиксирует usage, ответ возвращается
- Evidence: manual test procedure описана в README или CONTRIBUTING (не автоматический тест)

## Допущения

- Ollama запущен отдельно (не в процессе gateway)
- Ollama предоставляет OpenAI-совместимый `/v1/chat/completions` (стабильно с v0.1.x)
- Модель в Ollama уже загружена (`ollama pull <model>`)
- Для локальной разработки достаточно `http://localhost:11434` — HTTPS/TLS не требуется

## Критерии успеха

- SC-001 Запрос к локальному Ollama через gateway завершается за <5s (с учётом времени инференса модели)
- SC-002 0 изменений в существующих тестах провайдеров (OpenAI, Anthropic)

## Краевые случаи

- Ollama не запущен: 503 + диагностика
- Модель не найдена в Ollama: Ollama вернёт 400 — gateway проксирует оригинальную ошибку
- Пустой ответ от Ollama: корректная обработка пустого body
- Concurrent запросы: egress-клиент уже управляет пулом соединений
- Streaming с premature close: существующий streamSSE обрабатывает закрытие канала

## Открытые вопросы

1. Обрабатывать ли Ollama-specific поля в request body (`options`, `keep_alive`, `format`)? Решение: игнорировать — они передадутся как есть через proxy. Если Ollama их не поймёт — вернёт ошибку.
2. Нужна ли поддержка `api_key` для прокси-серверов перед Ollama (например, когда Ollama за reverse proxy с basic auth)? Решение: yes — `api_key` опционален, но если указан — используется как Bearer token.
