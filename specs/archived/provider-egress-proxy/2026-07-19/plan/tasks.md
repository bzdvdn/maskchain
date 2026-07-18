# provider-egress-proxy — Задачи

## T1: ProviderConfig — добавить поле ProxyURL

**AC:** AC-001

**Файлы:** `src/internal/infra/config/config.go`, `src/internal/infra/config/defaults.go`

**Действия:**
- [x] Добавить `ProxyURL string \`mapstructure:"proxy_url" yaml:"proxy_url"\`` в `ProviderConfig`
- [x] Маскировать ProxyURL в `MarshalLogObject` (serialize.go), если содержит credentials

**Trace:** `@sk-task provider-egress-proxy#T1.1`

---

## T2: proxy.go — новая функция proxyFuncFromURL + SOCKS5

**AC:** AC-002, AC-003, AC-005

**Файл:** `src/internal/adapters/egress/proxy.go`

**Действия:**
- [x] Добавить `proxyFuncFromURL(proxyURL string) (func(*http.Request) (*url.URL, error), error)`
- [x] Если `proxyURL == ""` — возвращать `proxyFunc()` (fallback на env)
- [x] Если `proxyURL` содержит `socks5://` — настроить `DialContext` через `golang.org/x/net/proxy`
- [x] Для `http://` и `https://` — возвращать фиксированный URL в proxy-функции

**Trace:** `@sk-task provider-egress-proxy#T2.1`

---

## T3: pool.go — передача proxyURL в NewTransport

**AC:** AC-002, AC-003, AC-005, AC-006

**Файл:** `src/internal/adapters/egress/pool.go`

**Действия:**
- [x] Изменить сигнатуру `NewTransport(cfg *Config, proxyURL string) (*http.Transport, error)`
- [x] В теле вызвать `proxyFuncFromURL(proxyURL)`, установить `tp.Proxy`
- [x] При `socks5://` схеме установить `tp.DialContext`
- [x] Все существующие call sites обновить с передачей `""` (сохраняет текущее поведение)

**Trace:** `@sk-task provider-egress-proxy#T3.1`

---

## T4: factory.go — передача ProxyURL в NewTransport

**AC:** AC-006

**Файл:** `src/internal/adapters/provider/factory.go`

**Действия:**
- [x] В `NewProviderClient` передать `pcfg.ProxyURL` вторым аргументом в `egress.NewTransport()`

**Trace:** `@sk-task provider-egress-proxy#T4.1`

---

## T5: Тесты

**AC:** AC-002, AC-003, AC-004, AC-005, AC-008

**Файл:** `src/internal/adapters/egress/egress_test.go`, `src/internal/adapters/egress/proxy_test.go` (новый)

**Действия:**
- [x] Создать `proxy_test.go`:
  - `TestProxyFromURL` — `proxyFuncFromURL("http://proxy:3128")` → RoundTrip через httptest proxy
  - `TestProxyEmptyURL` — `proxyFuncFromURL("")` → fallback на `proxyFunc()`
  - `TestSOCKS5Proxy` — `proxyFuncFromURL("socks5://localhost:1080")` → проверка DialContext
  - `TestExplicitNoProxy` — `proxy_url: ""` отключает proxy даже при установленном HTTP_PROXY
- [x] Существующий `TestCallViaProxy` должен продолжать проходить (AC-004)

**Trace:** `@sk-task provider-egress-proxy#T5.1`
