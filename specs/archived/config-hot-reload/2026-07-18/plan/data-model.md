---
status: no-change
reason: Hot-reload затрагивает только механизм загрузки и применения конфига в runtime. Ни одна сущность (Provider, Tenant, Route, etc.) не получает новых полей или методов. Все изменения — в инфраструктурном слое (config watcher) и сервисном слое (additive update-методы на существующих registry/selector).
---
