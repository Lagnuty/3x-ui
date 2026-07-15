# 3x-ui 2.8.11+ custom branch

This branch keeps the 3x-ui 2.8.11 panel style while backporting selected
features from newer 3x-ui releases and adding custom operational features.

## Versioning

Custom builds use:

```text
2.8.11.x.n
```

The current branch version is stored in `config/version`.

## Docker and release flow

- Branch: `3xui`
- Image: `ghcr.io/lagnuty/3x-ui:3xui`
- Version tags are also published by GitHub Actions.
- Docker runtime supports environment overrides for ports and selected Xray API
  ports so a container can be moved away from host port conflicts.

## API and authentication

- Added API token authentication through `Authorization: Bearer <token>`.
- Added `GET /panel/api/inbounds/listAll` for authenticated integrations that
  need to scan all inbounds regardless of the token owner's user scope.
- API tokens are stored as SHA-256 hashes; plaintext tokens are shown only once
  at creation.
- API tokens can be assigned to a panel user.
- Existing legacy tokens without an owner remain compatible.
- Added multi-user accounts with roles:
  - `admin`: panel and API access.
  - `api`: API-only access, blocked from web panel login.
- Panel sessions re-check the current user against the database, so disabled or
  downgraded users lose panel access.
- Added user action audit logs for login/logout, failed login, and mutating API
  or panel requests.
- Audit logs do not store passwords, API token plaintext, or request bodies.

## Server status API

- `GET /panel/api/server/status` is available to API-token callers.
- The status payload keeps the legacy fields and adds newer 3x-ui-style fields:
  - `panelVersion`
  - `diskIO.read`, `diskIO.write`
  - `diskTraffic.read`, `diskTraffic.write`
  - `netIO.pktUp`, `netIO.pktDown`
  - `netTraffic.pktSent`, `netTraffic.pktRecv`
- Network counters are summed from physical interfaces and skip common virtual
  interfaces such as loopback, Docker bridges, veth, tun/tap, WireGuard, and
  Tailscale.
- Status is refreshed once during server startup so the endpoint does not return
  `null` immediately after boot.

## Inbounds and protocols

- Added protocol/UI support work for newer protocols and modes while preserving
  the 2.8.11 interface style.
- Hysteria2 uses Xray's `hysteria` protocol with
  `streamSettings.hysteriaSettings.version = 2`.
- Hysteria2 share links use the `hysteria2://` URI scheme.
- Hysteria2 settings are normalized before Xray config generation so required
  transport fields are present.
- Updated bundled Xray core version support to run newer protocol configs.
- Added MTProto/FakeTLS helper logic.
- Added VLESS reverse/simple reverse fields.
- Added fallback and bridge-related data structures for newer topology features.

## Clients

- Added global Clients page.
- Added client groups.
- Added global client records separate from inbound JSON, with inbound
  attachment links.
- Added bulk client actions:
  - attach to inbounds
  - detach from inbounds
  - adjust expiry and traffic quota
  - reset traffic
  - delete clients
- Deleting an inbound now removes global client records that become orphaned.
- Bulk delete can remove already orphaned clients from the Clients page.
- Fixed JSON shape issues so clients/inbounds can be consumed without requiring
  frontend `JSON.parse` on already-object fields.

## Traffic, online status, and limits

- Added Hysteria2 inbound traffic accounting.
- Added Hysteria2 client traffic accounting using auth/email mapping.
- Added Hysteria2 online/offline client support where Xray stats expose the
  required counters.
- Added inbound-level Linux `tc` speed limits for the common protocols. The
  Docker image includes `iproute2`, compose files grant `NET_ADMIN`, and limits
  can be disabled with `XUI_TC_SPEED_LIMIT_ENABLE=false` or pointed at another
  interface with `XUI_TC_INTERFACE`.
- Added client speed limit fields:
  - `speedLimitUpMbps`
  - `speedLimitDownMbps`
- Hysteria2 speed limits are emitted into Hysteria settings.
- Docker builds now bundle the custom `Lagnuty/Xray-core`
  `v26.7.11-lagnuty.2` release, which adds dispatcher-level per-user bandwidth
  limits for shared inbounds.
- The panel writes and reads `bin/xray-version.txt`, so the dashboard/status API
  shows the custom core label (`26.7.11-lagnuty.1`) instead of only the upstream
  core version (`26.7.11`).
- The Xray version switcher prepends the custom release and downloads it from
  `Lagnuty/Xray-core`; upstream versions are still downloaded from
  `XTLS/Xray-core`.

## Nodes and bridges

- Added node management structures and UI.
- Added node heartbeat/status fields.
- Added chained/transitive node summary support.
- Added origin node GUID tracking for inbounds synced across node chains.
- Added selected/all inbound sync mode fields.
- Added bridge/fallback relationship models for newer routing topologies.

## Subscriptions and outbounds

- Added a new **Proxies** panel tab for local proxy URI generation.
- The Proxies tab can build share links, subscription text, QR codes, and Xray
  outbound JSON for VLESS, VMess, Trojan, Shadowsocks, Hysteria2, WireGuard,
  SOCKS, and HTTP.
- Proxy builder data is generated in the browser and is not stored in the panel
  database.
- Added outbound subscription model.
- Added subscription merge metadata.
- Added subscription JSON options, direct rules, fragments, noises, mux and
  related fields from newer subscription flows.
- Added custom share address strategy fields for generated links.

## Xray settings and runtime

- Added newer Xray config helper endpoints and UI fields.
- Added generated helpers for:
  - UUID
  - X25519
  - ML-DSA-65
  - ML-KEM-768
  - VLESS encryption auth options
  - ECH config
- Added safer geofile update allowlist behavior.
- Added Xray metrics and API port override environment support for Docker.

## Database compatibility

- Existing 2.8.11 users are migrated to `admin`.
- Existing user passwords are still bcrypt-migrated by the original seeder path.
- Existing API tokens remain usable.
- New tables are added through GORM AutoMigrate.
- Inbound JSON fields and client IP fields now support both legacy stringified
  JSON and object/array JSON payloads.

## Notes and known limits

- The branch intentionally keeps the 2.8.11 UI style instead of adopting the new
  frontend.
- API-only users cannot use the web panel by design.
- Speed limits for non-Hysteria protocols are stored but not yet enforced.
- `panelGuid` is not generated as a new persistent identity in this branch yet;
  node code can consume remote GUIDs, but local GUID persistence still needs a
  dedicated migration if strict new-version parity is required.
