# Porting Plan: v3.3.1 features into v2.8.11 UI

Source snapshots:

- Base UI/code: `D:\Documents\AIProjects\3x-ui 2.8.11+\upstream-v2.8.11`
- Latest upstream: `D:\Documents\AIProjects\3x-ui 2.8.11+\upstream-v3.3.1`
- Working copy: `D:\Documents\AIProjects\3x-ui 2.8.11+\work-2.8.11-plus`

Goal:

- Keep the v2.8.11 server-rendered Ant Design Vue interface and existing workflows.
- Backport functional additions from v3.3.1 without deleting existing v2.8.11 behavior.
- Avoid replacing the UI with the v3 React/Vite frontend.

Latest upstream detected:

- Repository: `https://github.com/MHSanaei/3x-ui.git`
- Base tag: `v2.8.11`
- Latest tag at download time: `v3.3.1`

Major upstream changes after v2.8.11:

Priority addition requested by user:

- Per-connection speed limit for each client/connection. This should be treated as a first-class client setting and preserved across add/update, bulk operations, attach/detach, import/sync, subscriptions, and Xray config generation.
- Bridge functionality from newer versions. Backport bridge workflows as old-UI features: model/storage, API, v2-style forms/tables, inbound/outbound config generation, validation, restart behavior, and compatibility with existing inbounds/clients/subscriptions.
- New old-UI proxy builder tab. Add a convenient interface for creating proxy links and Xray outbounds for common proxy types using domain names, IPv4, or IPv6 addresses.


- Project layout moved from top-level packages (`web`, `xray`, `database`, `sub`, `util`) to `internal/...`.
- Frontend moved from server-rendered HTML templates to a React/Vite SPA.
- New API documentation and generated OpenAPI schema.
- Central node management, node heartbeat, node traffic sync, and transitive node tree support.
- Client groups and bulk client workflows.
- More complete client service layer with global traffic, attach/detach, bulk adjust, lookup, locks, and sync helpers.
- MTProto support through an external `mtg` runtime manager.
- Hysteria/Hysteria2 inbound and outbound handling.
- Outbound subscription management.
- WARP and NordVPN integration improvements.
- Xray metrics and system metrics history.
- PostgreSQL support and SQLite/PostgreSQL migration/export helpers.
- CSRF middleware, login limiter, and additional security validation.
- Expanded tests around xray config, clients, nodes, subscriptions, database, and frontend adapters.

Porting order:

1. Backend compatibility layer
   - Keep the v2 package layout.
   - Backport only the service/controller/model changes required by selected features.
   - Preserve existing route names used by the v2 templates.

2. Low-risk protocol/config support
   - Update xray-core dependency and protocol constants.
   - Backport Hysteria/Hysteria2, MTProto, FinalMask, XHTTP, and outbound parser improvements where missing.
   - Add old-template form fields only where needed.

3. Client management
   - Priority: add per-connection speed limit support for every client/connection (per-connection speed limit).
   - Backport global client records, bulk create/delete/reset/adjust, attach/detach, group labels, and sync logic.
   - Add controls to the old inbounds/client tables instead of importing React pages.
   - Wire speed limits through client records, inbound settings JSON, old Ant Design Vue forms, API validation, and generated Xray config without changing the v2.8.11 layout.

4. Node management
   - Backport node model, node CRUD/probe service, controller, heartbeat jobs, and websocket broadcasts.
   - Add a v2-style `Nodes` page using the existing Ant Design Vue layout.

5. Xray/outbound integrations
   - Priority: backport bridge functionality from newer versions without importing the new frontend/backend wholesale.
   - Backport outbound subscriptions, WARP/Nord helpers, metrics endpoints, and route-testing support.
   - Add old-template modals for these workflows.
   - Add bridge model/API/UI support, bridge validation, and Xray config generation/restart handling in the v2.8.11 layout.

6. Database and security
   - Add migrations for new tables/columns.
   - Add PostgreSQL only after SQLite behavior remains compatible.
   - Add CSRF/login limiting in a way that does not break existing form submissions/API token calls.

7. Verification
   - Build on Windows for syntax only.
   - Run final build/tests on Ubuntu, because the projects are deployed on Ubuntu servers.
   - Smoke-test login, inbounds, clients, xray settings, subscriptions, and restart behavior.

Important constraint:

- Do not copy the `frontend/` React SPA into the working product as the main UI. It can be used as a reference for API behavior and form fields only.

Progress:

- Added v3.3.1 model foundation to the v2 package layout:
  `Node`, `ApiToken`, `ClientRecord`, `ClientInbound`, `ClientGroup`,
  `InboundFallback`, `NodeClientTraffic`, `ClientGlobalTraffic`, and
  `OutboundSubscription`.
- Kept the v2 `Inbound.AllTime` field for old UI/runtime compatibility.
- Added `AutoMigrate` coverage for the new tables.
- Added a minimal v2-compatible `ClientService`.
- Added client group backend APIs under `/panel/api/clients/groups...`.
- Added inbound-client synchronization from old `inbounds.settings.clients[]`
  into the new `clients` and `client_inbounds` tables.
- Added a v2-compatible API token service using hashed tokens, bearer-token
  authentication for `/panel/api`, and old session authentication as fallback.
- Added API token management endpoints under `/panel/setting/apiTokens...`.
- Added a v2-style API Tokens panel inside Settings -> Security. The plaintext
  token is shown only immediately after creation.
- Added initial global client API endpoints under `/panel/api/clients` for
  listing synchronized clients, reading one client with inbound attachments,
  reading/updating traffic, IP records, online clients, and last-online data.
- Added a v2-style global `Clients` page at `/panel/clients` with search,
  attachment/group/status/traffic display, traffic update, and IP record view.
- Added v2-compatible client attach/detach APIs and bulk attach/detach. These
  update old inbound settings JSON and the new `client_inbounds` table while
  preserving the v2 `client_traffics.email` model.
- Added attach/detach controls to the global `Clients` page. Operations mark
  Xray as needing restart because v2's live API/stat model is still email-based.
- Added resetTraffic and bulkResetTraffic endpoints to the global clients API,
  plus a per-client reset button in the `Clients` page.
- Added v3-style in-memory login rate limiting: 5 failed attempts per IP and
  username in 5 minutes triggers a 15 minute cooldown. Also stopped logging or
  sending failed-login passwords to Telegram notifications.
- Added the first v2-compatible outbound subscription layer: CRUD, ordering,
  SSRF-conscious HTTP/HTTPS URL validation, manual refresh, JSON array or
  `{outbounds:[...]}` parsing, tag prefixing, and storage in
  `outbound_subscriptions`.
- Added old Ant Design Vue controls for outbound subscriptions inside the
  Xray -> Outbounds tab. Full v3 link parsing and automatic config merge are
  still a separate follow-up.
- Wired fetched outbound subscription JSON into `XrayService.GetXrayConfig()`.
  Active subscription outbounds are merged with manual template outbounds on
  restart/reload, with `Prepend` ordering and duplicate tag skipping.
- Added a cron job that refreshes enabled outbound subscriptions every 5
  minutes when their per-subscription update interval is due, then marks Xray
  as needing restart so the existing restart loop reloads the merged config.
- Added a minimal v2 share-link parser for outbound subscriptions. Refresh now
  accepts base64 or plaintext newline subscriptions with `vmess://`,
  `vless://`, `trojan://`, and `ss://` links in addition to JSON outbounds.
- Added global clients `bulkAdjust` and `bulkDel` APIs. They update the new
  `clients` table, old inbound settings JSON, traffic rows, IP rows, and mark
  Xray for restart when runtime config changes.
- Added row selection and bulk adjust/reset/delete controls to the old
  v2-style `Clients` page.
- Added v2-compatible node management:
  - `/panel/api/nodes/list`, `get`, `add`, `update`, `del`, `setEnable`,
    `test`, and `probe`.
  - `/panel/nodes` old Ant Design Vue page with status, metrics, versions,
    counts, enable switch, add/edit/delete, test, and probe.
  - Node health checks call the remote panel's existing
    `/panel/api/server/status` endpoint with bearer-token auth.
  - TLS verify/skip/pin modes and private-address blocking are supported, with
    an explicit `Allow private/internal address` checkbox for trusted Ubuntu
    server networks.
  - Added an automatic heartbeat job that probes enabled nodes every minute and
    stores online/offline status, latency, xray/panel versions, CPU, memory,
    uptime, and last error.
- Added an old-UI Xray -> Proxy Builder tab for creating proxy share links and
  outbound JSON directly in the interface. It supports VMess, VLESS, Trojan,
  Shadowsocks, Hysteria2, HTTP, SOCKS, and WireGuard; can copy generated links
  or JSON; can import supported share links into the existing outbound modal;
  and can add generated outbound JSON directly to the manual outbounds list.

Remaining high-risk items:

- Full v3 node runtime sync/deploy/transitive tree support is not copied yet.
  It depends on v3's new `internal/web/runtime` manager and would pull in a
  large part of the new backend the user does not want. The current backport
  deliberately keeps node management to CRUD/probe/status so the old 2.8.11
  runtime stays intact.
- Outbound subscription share-link parsing is still minimal compared with v3.
  It supports `vmess://`, `vless://`, `trojan://`, and `ss://`, but not every
  v3 protocol/detail such as Hysteria2/WireGuard/XHTTP edge fields.



Bridge progress:

- Panel egress bridge implemented: selected outbound/balancer injects a loopback SOCKS inbound tagged panel-egress, prepends a routing rule, exposes the choice in old Xray -> Outbounds UI, and outbound subscription fetches use this bridge when Xray is running.
