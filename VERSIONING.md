# 3xui Branch Versioning

This fork keeps the original `v2.8.11` UI line and uses a local version format:

```text
2.8.11.x.n
```

- `2.8.11` stays fixed to show the upstream base.
- `x` is the local feature line.
- `n` is incremented for every local change set.

Current version is stored in `config/version`.

To bump the patch counter on Ubuntu:

```bash
./scripts/bump-version.sh
```

Docker build from a GitHub branch:

```bash
docker build \
  -f Dockerfile.remote \
  --build-arg XUI_REPO=https://github.com/Lagnuty/3x-ui.git \
  --build-arg XUI_REF=3xui \
  -t 3xui:2.8.11.1.0 .
```

Docker Compose:

```bash
XUI_REPO=https://github.com/Lagnuty/3x-ui.git XUI_REF=3xui \
docker compose -f docker-compose.remote.yml up -d --build
```
