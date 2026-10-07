# Технический справочник UI 3x-ui 2.9.4+23wl2

Документ описывает новые элементы интерфейса, добавленные для Xray
v26.4.25–v26.9.30. Имена JSON приведены в том виде, в котором панель передаёт
их Xray. Пустые необязательные значения обычно не должны фиксировать старый
default: решение остаётся за установленной версией ядра.

## XHTTP inbound и outbound

Расположение: редактор inbound/outbound → `Transport = XHTTP`.
JSON: `streamSettings.xhttpSettings`.

### Основные и upload-поля

| Поле UI | JSON | Тип / значения | Начальное значение и поведение |
|---|---|---|---|
| Host | `host` | string | Пустая строка. HTTP Host для транспорта. |
| Path | `path` | string | `/`. Путь XHTTP. |
| HTTP headers | `headers` | object `name: value` | Пары добавляются кнопкой `+`. На outbound могут переопределить автоматически выбранные ядром headers. |
| Mode | `mode` | `auto`, `packet-up`, `stream-up`, `stream-one` | Определяет доступность зависимых полей. |
| Max Buffered Upload | `scMaxBufferedPosts` | integer | `30`; показывается для `packet-up`. |
| Max Upload Size | `scMaxEachPostBytes` | integer/range string | `1000000`; показывается для `packet-up`. |
| Min Upload Interval | `scMinPostsIntervalMs` | integer/range string | Пусто = default текущего core. Старое принудительное `30` удаляется миграцией. |
| Stream-Up Server | `scStreamUpServerSecs` | integer/range string | `20-80`; только `stream-up`. |
| No gRPC Header | `noGRPCHeader` | boolean | `false`; доступно для `stream-up`/`stream-one`. |
| No SSE Header | `noSSEHeader` | boolean | `false`. |
| Server Max Header Bytes | `serverMaxHeaderBytes` | integer ≥ 0 | `0` = default core. |

### Session ID и размещение данных

| Поле UI | JSON | Значения | Правило |
|---|---|---|---|
| Session ID Placement | `sessionIDPlacement` | empty, `path`, `header`, `cookie`, `query` | Пусто = `path` по умолчанию ядра. |
| Session ID Key | `sessionIDKey` | string | Показывается, если placement не `path`; пример `x_session`. |
| Session ID Table | `sessionIDTable` | string | Необязательная таблица символов ID. |
| Session ID Length | `sessionIDLength` | integer ≥ 0 | `0`/пусто = default core. |
| Sequence Placement | `seqPlacement` | empty, `path`, `header`, `cookie`, `query` | Пусто = `path`. |
| Sequence Key | `seqKey` | string | Пример `x_seq`; не нужен для `path`. |
| Uplink Data Placement | `uplinkDataPlacement` | empty, `body`, `header`, `cookie`, `query` | Только `packet-up`; пусто = `body`. |
| Uplink Data Key | `uplinkDataKey` | string | Показывается при размещении не в body. |
| Uplink Chunk Size | `uplinkChunkSize` | integer ≥ 0 | `0` = без лимита; для data placement не в body. |
| Uplink HTTP Method | `uplinkHTTPMethod` | empty, `POST`, `PUT`, `GET` | Пусто = `POST`; `GET` только для `packet-up`. |

Миграция выполняется автоматически:

- `sessionPlacement` → `sessionIDPlacement`;
- `sessionKey` → `sessionIDKey`;
- если одновременно есть старое и новое поле, приоритет имеет новое.

### Padding/obfuscation

| Поле UI | JSON | Тип / значения | Default |
|---|---|---|---|
| Padding Bytes | `xPaddingBytes` | range string | `100-1000`. |
| Padding Obfs Mode | `xPaddingObfsMode` | boolean | `false`. |
| Padding Key | `xPaddingKey` | string | Пример `x_padding`; видно при включённом obfs. |
| Padding Header | `xPaddingHeader` | string | Пример `X-Padding`. |
| Padding Placement | `xPaddingPlacement` | empty, `queryInHeader`, `header`, `cookie`, `query` | Пусто = `queryInHeader`. |
| Padding Method | `xPaddingMethod` | empty, `repeat-x`, `tokenish` | Пусто = `repeat-x`. |

Поля, для которых нет стандартного share-link параметра, помещаются в JSON
`extra=`. `xPaddingBytes` также выводится как `x_padding_bytes` там, где формат
клиента его понимает.

### XMUX

Переключатель `XMUX` управляет наличием объекта `xhttpSettings.xmux`.
Выключенный XMUX означает «не передавать объект, использовать решение core».

| Поле UI | JSON | UI default |
|---|---|---|
| Max Concurrency | `xmux.maxConcurrency` | `16-32` |
| Max Connections | `xmux.maxConnections` | `0` |
| Max Reuse Times | `xmux.cMaxReuseTimes` | `0` |
| Max Request Times | `xmux.hMaxRequestTimes` | `600-900` |
| Max Reusable Secs | `xmux.hMaxReusableSecs` | `1800-3000` |
| Keep Alive Period | `xmux.hKeepAlivePeriod` | `0` |

Значения являются строками диапазонов там, где Xray допускает диапазон.
Сохранённый старым интерфейсом полный набор defaults удаляется миграцией,
чтобы изменения fallback в v26.7.28+ применялись ядром. Явно изменённый XMUX
не удаляется.

## gRPC / CDN real client IP

Расположение: inbound → stream → `Sockopt`.
JSON: `streamSettings.sockopt.trustedXForwardedFor`.

`Trusted X-Forwarded-For` — массив имён HTTP-заголовков. Доступны быстрые
пресеты:

- Cloudflare: `CF-Connecting-IP`, `True-Client-IP`, `X-Forwarded-For`;
- Reverse Proxy: `X-Forwarded-For`, `X-Real-IP`.

Поле работает для WS, HTTPUpgrade, XHTTP и, начиная с v26.6.22, gRPC. Вносить
сюда заголовок можно только если ближайший proxy доверенный, иначе клиент
сможет подменить IP.

## TUN inbound

Расположение: inbound → `Protocol = TUN`.
JSON: `inbound.settings`.

| Поле UI | JSON | Тип / ограничения | Default / зависимость |
|---|---|---|---|
| Interface Name | `name` | string | `xray0`. |
| MTU | `mtu` | integer 1–9000 | `1500`; текущий Xray принимает одно число. |
| Gateway | `gateway` | string array | IPv4/IPv6 CIDR. |
| DNS | `dns` | string array | Адреса DNS. |
| User Level | `userLevel` | integer ≥ 0 | `0`. |
| Auto Routing Table | `autoSystemRoutingTable` | string array | Примеры: `auto`, `main`, `100`, `vpn`. |
| Auto Outbounds | `autoOutboundsInterface` | string | `auto`, либо `eth0`, `ens3`, `Ethernet`. |
| System DNS to Gateway | `autoSystemDnsToGateway` | boolean | Linux v26.9.30+; требует непустой `gateway`. |
| Windows WFP Leak Block | `autoSystemWfpBlockLeak` | array: `dns`, `misconfigtun` | Windows v26.9.30+; требует auto routing, вариант `dns` также требует DNS. |

Кнопки `Auto`, `Linux`, `Windows` заполняют routing table/interface типовыми
значениями. Старый массив `mtu` мигрирует в одно число — первый элемент.

Блок состояния над полями показывает:

- Linux: наличие `/dev/net/tun`, root, `CAP_NET_ADMIN`, `CAP_NET_RAW`;
- Windows: elevated token;
- итоговый `ready` и текст причины, если TUN запустить нельзя.

## Hysteria2 VLESS route

Расположение: Xray settings → Routing → правило → `VLESS Route`.
JSON: `routing.rules[].vlessRoute`.

Тип — строка или CSV со значениями портов/диапазонов, например
`53,443,1000-2000`. Правило связывается с `inboundTag`. Для Hysteria2 значение
экспортируется:

- в share link как `vlessRoute`;
- в Mihomo YAML как `vless-route`.

Старые `network: hysteria2` и `hy2Settings` при загрузке преобразуются в
`network: hysteria` и `hysteriaSettings`.

## FinalMask editor

Расположение: inbound/outbound → stream → `TCP Masks`, `UDP Masks`,
`QUIC Params`. JSON: `streamSettings.finalmask`.

### TCP mask types

- `fragment`: `packets`, `length`, `delay`, `maxSplit`;
- `header-custom`: группы `clients`, `servers`, `errors`; элемент содержит
  `type` (`array`, `str`, `hex`, `base64`), `packet`, `delay`, а для array —
  `rand` и `randRange`;
- `sudoku`: `password`, `ascii`, `customTable`, `customTables`, `paddingMin`,
  `paddingMax`;
- `xmc`: Minecraft-compatible профиль, описанный ниже.

### XMC

JSON: `finalmask.tcp[].type = "xmc"`, параметры в `settings`.

| Поле UI | JSON | Проверка перед сохранением |
|---|---|---|
| Hostname | `hostname` | Необязательная строка. |
| Password | `password` | Обязательная непустая строка. |
| Profiles | `profiles` | Минимум один полностью валидный профиль. |
| Username | `profiles[].username` | 3–16 латинских букв, цифр или `_`. |
| UUID | `profiles[].uuid` | Валидный UUID. |
| Textures Value | `profiles[].texturesValue` | Обязательная непустая строка. |
| Textures Signature | `profiles[].texturesSignature` | Обязательная непустая строка. |

Неполный XMC блокируется в UI. Если он пришёл из старой базы напрямую,
backend удаляет маску и возвращает migration warning, чтобы Xray не отказался
запускаться.

### UDP mask types и поля

Набор зависит от протокола: Hysteria, WireGuard, mKCP и Shadowsocks.

- `salamander`: пароль Hysteria2 obfs;
- `mkcp-aes128gcm`, `mkcp-aes256gcm`, `mkcp-chacha20poly1305`,
  `mkcp-simplexor`, `mkcp-none`, `mkcp-legacy`;
- `header-dns`, `header-http`, `header-srtp`, `header-utp`, `header-wechat`,
  `header-wireguard`;
- `xicmp`;
- `xdns`;
- `noise`, включая packet type `exp` в v26.9.30.

### XDNS

| Поле UI | JSON | Тип / ограничения | Default |
|---|---|---|---|
| Domain | `domains[].name` | string | Пусто. |
| Length Limit | `domains[].lenLimit` | integer 1–255 | `255`. |
| Label Limit | `domains[].labelLimit` | integer 1–63 | `63`. |
| DNS Types | `domains[].types` | integer array | Например `1,28` для A/AAAA. |
| EDNS0 | `domains[].edns0` | integer ≥ 0 | `0`. |
| Resolver Type | `resolvers[].type` | `udp` или `tcp` | `udp`. |
| Resolver Address | `resolvers[].settings.addr` | `host:port` | Пример `1.1.1.1:53`. |
| Extra Poll | `extraPoll` | integer 0–3 | `0`. |

Старый `domains: ["example.org"]` превращается в объект с лимитами 255/63,
пустыми types и EDNS0=0.

### QUIC / UDP hop

`QUIC Params` показываются для Hysteria и XHTTP/3.

| Поле UI | JSON | Значения / ограничения |
|---|---|---|
| Congestion | `quicParams.congestion` | `reno`, `bbr`, `brutal`, `force-brutal`. |
| Brutal Up/Down | `brutalUp`, `brutalDown` | Для brutal-вариантов, значение от 65537. |
| UDP Hop | наличие `quicParams.udpHop` | Переключатель. |
| Hop Ports | `udpHop.ports` | Строка, например `20000-50000`. |
| Hop Interval | `udpHop.interval` | Не менее 5 секунд. |
| Max Idle Timeout | `maxIdleTimeout` | 4–120 секунд. |
| Keep Alive Period | `keepAlivePeriod` | 2–60 секунд. |

Для XHTTP/3 UI предупреждает, что клиент должен поддерживать тот же udpHop, а
маршрут должен быть согласован с `sockopt.dialerProxy`.

## REALITY и post-quantum compatibility

Расположение: inbound → Security → REALITY.
JSON основного сервера: `streamSettings.realitySettings`.

| Поле UI | JSON / назначение | Тип / поведение |
|---|---|---|
| uTLS | client `fingerprint` | Рекомендуется `chrome`; другие значения получают compatibility warning. |
| Min Client Ver | `minClientVer` | Version string. Пусто = минимум не задан. |
| Max Client Ver | `maxClientVer` | Необязательная верхняя граница. |
| Strict modern clients | preset | Ставит `26.9.8`. |
| Compatibility | preset | Очищает `minClientVer`. |
| X25519MLKEM768 | client-export control | Default `true`; включает capability в Mihomo. |
| ML-KEM-768 Seed | диагностический tool input | Base64 seed; это не REALITY private key. |
| ML-KEM-768 Client | tool output | Read-only client key. |
| Generate | server action | Запускает `xray mlkem768`. |
| Verify seed | server action | Повторно вычисляет/проверяет введённый seed. |

Старое автоматически сохранённое `minClientVer: "26.3.27"` удаляется. Другой
явно заданный минимум сохраняется.

При включённой ML-KEM совместимости Mihomo получает:

```yaml
reality-opts:
  support-x25519mlkem768: true
```

Если fingerprint клиента пустой, Mihomo получает `client-fingerprint: chrome`.
Raw `vless://` этот capability-флаг не переносит.

## TLS pinning и peer-name verification

Расположение:

- inbound → TLS → `Client verification / subscription overrides`;
- Xray outbound → TLS settings.

JSON: `tlsSettings.verifyPeerCertByName` и
`tlsSettings.pinnedPeerCertSha256`.

| Поле UI | Тип / формат | Экспорт |
|---|---|---|
| Verify Peer Cert By Name | hostname или CSV имён | Share-link `vcn`; сохраняется в client JSON. |
| Pinned Peer Cert SHA256 | один или несколько 64-символьных hex SHA-256 через запятую | Share-link `pcs`; сохраняется в client JSON. |
| TLS ping target | `hostname[:port]` | Ввод для получения pin; схемы URL и пути запрещены. |

Кнопка получения pin запускает `xray tls ping`, берёт leaf SHA-256 из проверки
с SNI и нормализует hex в нижний регистр. `allowInsecure` больше не экспортирует
отключение проверки сертификата.

Legacy-поля `pinnedPeerCertificateSha256` и
`pinnedPeerCertificateChainSha256` преобразуются в
`pinnedPeerCertSha256`.

## Outbound HTTP headers и User-Agent

Расположение: Xray outbound → RAW HTTP или XHTTP.

| Поле UI | JSON | Поведение |
|---|---|---|
| User-Agent override | `headers.User-Agent` | Пусто = Xray генерирует текущий Chrome UA. |
| HTTP headers | `headers` | Список name/value; одинаковые имена преобразуются в формат, ожидаемый транспортом. |

Для RAW HTTP задание Host или пользовательского header заменяет стандартный
набор request headers целиком, поэтому при необходимости UA следует указать
явно. В share links headers переносятся через `extra=`.

## VLESS reverse UI

### Публичная сторона

Расположение: VLESS inbound → client → `Reverse Tag`.
JSON клиента: `clients[].reverse.tag`.

UUID такого клиента используется как отдельный reverse account и не должен
переиспользоваться для обычного proxy traffic.

### Приватная/bridge сторона

Расположение: VLESS outbound → `Reverse Tag`.
JSON: `outbound.settings.reverse.tag`.

Дополнительные поля:

- `Reverse traffic sniffing` → `reverse.sniffing.enabled`;
- `Destination override` → `reverse.sniffing.destOverride`, значения `http`,
  `tls`, `quic`, `fakedns`;
- `Live reverse tunnel test` принимает `tcp://host:port`, `http://...` или
  `https://...`, показывает задержку, HTTP status или текст ошибки.

В Xray settings → Advanced отображается мастер миграции старых top-level
`reverse.bridges`/`reverse.portals` в VLESS reverse tags.

## Domain matcher / cache diagnostics

Расположение: Xray settings → Routing, верхняя диагностическая карточка.

| Элемент UI | Источник / значение |
|---|---|
| Matcher / cache mode | `auto-mph`; поле read-only, поскольку кэш управляется Xray. |
| Cache mode | `shared in-memory weak cache`. |
| Lifecycle | очищается и строится при restart Xray. |
| Status | `stopped`, `starting`, `ready`. |
| Startup duration | время до сообщения о готовности core. |
| HIT / MISS | счётчики из debug log Xray. |
| Slow start | warning при startup ≥ 3000 ms. |
| Geosite / GeoIP | путь, наличие, размер, modification time, ошибка. |
| Rebuild matcher cache | restart Xray и повторное чтение diagnostics. |

Панель не записывает устаревший `domainMatcher` в Xray JSON.

## Deprecated configs и conversion wizard

Расположение: Inbounds.

- новый фильтр `Deprecated`;
- оранжевая метка и popover с конкретными причинами;
- action `Convert to VLESS Reality/Vision`;
- предупреждения для VMess, Trojan/VLESS без Flow, legacy Shadowsocks,
  `allowInsecure` и gRPC на новых core.

Мастер создаёт современную конфигурацию, но перед сохранением оставляет
оператору возможность проверить порт, transport, REALITY target/SNI, Flow и
клиентов.

## Xray release switcher

Расположение: главная страница → обновление Xray.

Перед кнопкой установки показываются:

- compatibility matrix для v26.4.25, v26.6.22, v26.6.27, v26.7.28,
  v26.9.8, v26.9.9 и v26.9.30;
- `Affected inbounds` с ID, remark, protocol и списком причин;
- результат config generation dry-run, число inbound и размер JSON;
- общие warnings: REALITY/Shadowrocket, reverse regression, gRPC reconnect.

При установке candidate binary сначала проверяется командой
`xray run -test -c <generated-config>`. Текущий binary переименовывается в
rollback backup только после успешного dry-run. Если новый core не стартует,
backup автоматически возвращается.

## Subscription mapping

| Возможность | Raw link | Mihomo | sing-box 1.14.1 |
|---|---|---|---|
| XHTTP стандартные поля | query + `extra=` | `xhttp-opts` | Не поддерживается: явная ошибка. |
| FinalMask | `fm`/`extra=` | дополнительные поля | Не поддерживается: явная ошибка. |
| TLS pin/name | `pcs`, `vcn` | TLS overrides | Поддерживаемые TLS-поля JSON. |
| REALITY ML-KEM capability | Не переносится | `support-x25519mlkem768: true` | Зависит от клиента, не подменяется невалидным полем. |
| Hysteria vlessRoute | `vlessRoute` | `vless-route` | Только если формат клиента поддерживает поле. |
| HTTP headers | `extra=` | transport headers | Поддерживаемые transport headers. |

Если целевой формат не умеет представить обязательную серверную настройку,
генератор должен вернуть ошибку совместимости, а не молча создать профиль с
другим transport/security.
