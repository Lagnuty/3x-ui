# 3x-ui 2.9.4+26 — Xray v26.10.21

Это продолжение панели 2.x для тех, кто не хочет переходить на 3.x. Сборка
Docker публикуется GitHub Actions для Ubuntu/Linux (amd64, arm64, arm/v7, 386)
под тегами `ghcr.io/lagnuty/3x-ui:2.9.4-26`, `:2.9.4` и `:latest` **после
успешного завершения workflow**. `docker-compose.yml` использует `:latest`.

## Почему обновление срочное

Xray v26.10.10 и v26.10.17 отозваны: встроенный fallback-набор доверенных
корней мог принимать скомпрометированные промежуточные сертификаты. v26.10.21
удаляет соответствующие корни и собран с Go 1.27.3. Панель и встраиваемый
Go-модуль Xray переключены на v26.10.21; Docker использует Go 1.27.3 и
скачивает официальный бинарник этого выпуска. На устройствах с Xray-клиентом
его нужно обновить **отдельно**: установка панели не обновляет удалённые клиенты.
Это не свидетельство того, что ваш трафик был перехвачен; риск касается
проверки TLS-сертификатов при наличии подходящей атаки.

Источники: [релиз Xray v26.10.21](https://github.com/XTLS/Xray-core/releases/tag/v26.10.21),
[исправление набора корней](https://github.com/XTLS/Xray-core/pull/7212).

## Поля и изменения интерфейса

| Где | Новое поле / управление | Конфигурация Xray и смысл |
|---|---|---|
| Обновление ядра → Compatibility matrix | v26.10.21 | Контрольная точка срочного TLS-обновления, FinalMask filter и XDRIVE H3/XMUX. |
| Обновление ядра → предупреждения | текущая версия v26.10.10–v26.10.17 | Явное напоминание обновить сервер и Xray-клиенты, даже если выбрана безопасная целевая версия. |
| Inbound → FinalMask → TCP/UDP mask | `filter` | Маска фильтра на входе; поля `allowedSourceIPs` и `blockedSourceIPs` принимают CIDR/`geoip:`. Требуется core v26.10.21+. |
| Inbound/outbound VLESS → XDRIVE | `Template` и `Template JSON` | JSON-объект `xdriveSettings.template`; доступны двухшаговые операции с `hrefPath` и `hrefMethod`. |
| Inbound/outbound VLESS → XDRIVE | `Use H3` / `Use H2` | Выставляют `tlsSettings.alpn` строго в `["h3"]` / `["h2"]`; требуется TLS. |
| Inbound/outbound VLESS → XDRIVE | `XMUX override` | Включает объект `xdriveSettings.xmux`; при выключении ядро выбирает defaults. |
| XDRIVE → XMUX | `Max connections`, `Max concurrency`, `Client max reuse`, `H max requests`, `H reusable seconds`, `H keep-alive seconds` | Ключи `maxConnections`, `maxConcurrency`, `cMaxReuseTimes`, `hMaxRequestTimes`, `hMaxReusableSecs`, `hKeepAlivePeriod`. Положительные `maxConnections` и `maxConcurrency` не задавайте одновременно. |
| XDRIVE H3 → FinalMask | `UDP Masks`, `QUIC Params` | В Google Drive H3 допустима только UDP-маска `noise`; congestion — BBR/Reno, не Brutal. |

`filter` — серверное правило, которое не копируется в клиентские JSON и
подписки. Ссылки `vless://` не описывают XDRIVE; секреты Google Drive в них
не экспортируются. Для такого транспорта настройте совместимый Xray-клиент
отдельно. Полный список параметров и ограничений — в
[техническом справочнике](XRAY_26_UI_FIELDS_RU.md).

## Проверки

- `go test ./web ./web/service ./sub ./database/model ./xray` на Go 1.27.3;
- Node-тесты round-trip моделей XDRIVE inbound/outbound;
- `xray run -test` на официальном бинарнике v26.10.21 для тестового
  FinalMask filter-конфига;
- Linux multi-arch Docker build выполняет GitHub Actions. Публикация тегов
  подтверждается только успешным workflow, не самим наличием этого файла.

Сборка и тестирование конфига не заменяют функциональный тест удалённого
XDRIVE-соединения: для него нужны реальные ключи, TLS-сертификат и хранилище.
