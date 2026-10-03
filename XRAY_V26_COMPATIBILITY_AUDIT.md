# Xray v26 compatibility audit

Audit target: Xray `v26.9.30` (`b26a91d`) and the compatibility work described
in items 1-22. The audit is intentionally split between schema validation,
migration tests, subscription validation, and platform build checks: a config
accepted by Xray alone does not prove that old saved data or client exports work.

## Coverage

| # | Area | Verified implementation |
|---|---|---|
| 1 | XHTTP session ID fields | Old keys migrate to `sessionID*`; inbound/outbound UI, links, imports, and Mihomo extras use the new names. |
| 2 | XHTTP/3 and UDP hop | QUIC/UDP hop fields and compatibility hints are present; the current Xray fixture exercises the schema. |
| 3 | gRPC trusted XFF | gRPC exposes `trustedXForwardedFor`; release diagnostics also warn about the v26.9.30 reconnect risk and gRPC deprecation. |
| 4 | WireGuard workers | `workers` and `num_workers` are removed during migration and never emitted. Removed v26.9.30 WireGuard fields are normalized too. |
| 5 | TUN routing | Routing table/interface plus v26.9.30 DNS/WFP fields are modeled and validated. Linux root/TUN/capability and Windows elevation readiness are reported. |
| 6 | TUN counters | Nested and legacy Xray counter names are accepted, so TUN inbounds use the normal traffic display/reset pipeline. |
| 7 | Hysteria VLESS route | Routing UI, share link and Mihomo `vless-route` export are present; old Hysteria2 shapes migrate to the current Hysteria model. |
| 8 | FinalMask XMC | Editor, strict profile/UUID/secret/texture validation, migration stripping, warnings and exports are covered. |
| 9 | FinalMask UDP | XICMP, current XDNS, header/mKCP variants and legacy-shape migration are covered by the real Xray fixture and unit tests. |
| 10 | XHTTP padding/obfs | Padding/obfs fields, experimental hints and `extra=` fallback are exported and imported. |
| 11 | XHTTP XMUX defaults | UI exposes strategy fields; old saved core defaults are removed while explicit custom values are preserved. |
| 12 | REALITY ML-KEM | Generator/parser/length validation, UI compatibility state and Mihomo `support-x25519mlkem768: true` are tested. Raw `vless://` limitation is shown. |
| 13 | REALITY minimum client | Empty means no minimum, legacy built-in values migrate away, explicit values survive, and strict/compatibility presets are available. |
| 14 | REALITY fingerprints | Compatibility hints are shown; Mihomo defaults an absent fingerprint to `chrome`; security overrides remain visible. |
| 15 | TLS pinning | SHA-256 leaf pin and verification name are modeled and exported. `xray tls ping` output parsing is tested; deprecated insecure export is blocked. |
| 16 | Dynamic HTTP UA | Outbound XHTTP/HTTP headers can override UA; leaving it empty delegates the dynamic Chrome UA to Xray. Headers survive link fallback. |
| 17 | MPH matcher cache | UI reports automatic MPH lifecycle, HIT/MISS, startup duration and geodata errors; rebuild restarts the core-managed cache. |
| 18 | VLESS reverse | UI, version-range warnings, legacy mapping migration and a live TCP/HTTP tunnel probe are present. |
| 19 | Deprecations | Inbound warnings/filter and modern VLESS REALITY/Vision conversion cover VMess, missing Flow, legacy Shadowsocks and `allowInsecure`. |
| 20 | Release switcher | Version matrix includes every requested checkpoint through v26.9.30, affected inbounds, generated-config dry-run, candidate `run -test`, rollback and REALITY/Shadowrocket warning. |
| 21 | Shares/subscriptions | XHTTP/FinalMask/REALITY parameters, Mihomo extras, `extra=` fallback and version/client compatibility decisions are covered. Unsupported official sing-box extensions fail explicitly instead of producing invalid JSON. |
| 22 | Migration layer | XHTTP, WireGuard, REALITY, XMC, TLS pinning and Hysteria/Hysteria2 migrations are centralized and unit-tested. |

## Executed checks

- Xray `v26.9.30` real binary: `run -test` accepts
  `testdata/xray-v26.9.30-compat.json`.
- Mihomo `v1.19.32`: real binary accepts
  `testdata/mihomo-v1.19.32-compat.yaml`.
- sing-box `v1.14.1`: real binary accepts the supported fixture and the
  generator rejects XHTTP/FinalMask because official sing-box does not support
  those Xray transports.
- Go compatibility suites pass for `database/model`, `sub`, `web/service`, and
  `xray`; `go vet ./...` passes.
- JavaScript syntax checks pass for the inbound, outbound, and DB inbound
  models.
- A `linux/amd64`, `CGO_ENABLED=0` panel binary builds successfully.
- No `.pyc` files or `__pycache__` directories are present.

The repository-wide test run has two unrelated SQLite integration failures on
the local portable Windows Go toolchain because it has no CGO compiler. The
Ubuntu Docker builder enables CGO and installs `build-base`; those two tests
must still be run in that target image or CI when a Docker daemon is available.

## Scope note

The requested 1-22 compatibility scope is covered. Xray v26.9.30 also contains
newer transports such as XDRIVE and MASQUE; they were not in the requested
scope and do not yet have dedicated panel editors or subscription formats.
They should be treated as a separate feature patch, not as silently supported
options.
