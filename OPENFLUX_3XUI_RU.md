# OpenFlux в 3x-ui 2.9.4+23wl1

Эта ветка добавляет нативный раздел **OpenFlux** для профилей Yandex Docs / Boards / vyandex и мобильного API L-VPN.

## Интерфейс

- Страница панели: `/panel/openflux`.
- Поддержанные транспорты: `yandex`, `boards`, `vyandex`.
- Для каждого профиля сохраняются название, `serverId`, URL, режим `l4/l3`, кодек `batched`, debug-уровень, путь к файлу ключа и флаг включения.
- URL в таблице, systemd preview и логах маскируется, чтобы не светить рабочую ссылку в интерфейсе.

## Установка и обновление бинаря

В блоке **Бинарный файл OpenFlux** интерфейс показывает текущую версию `/usr/local/bin/openflux`, HEAD-коммит GitHub и позволяет установить или обновить бинарь из GitHub.

Серверный установщик делает:

1. Проверяет, что панель запущена на Linux.
2. Проверяет наличие `git` и `go`; если их нет на Ubuntu/Debian-сервере,
   пытается установить `git golang-go` через `apt-get`.
3. Клонирует `https://github.com/p1neappleXpress/OpenFlux.git`.
4. По желанию переключается на указанный `ref`: `HEAD`, branch, tag или commit.
5. Собирает статический linux-amd64 бинарь командой `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -trimpath`.
6. Проверяет, что кандидат отвечает на `--version` или `version`.
7. Делает rollback-backup текущего `/usr/local/bin/openflux`.
8. Ставит новый бинарь в `/usr/local/bin/openflux`.
9. Переприменяет включённые `openflux-<id>.service`.
10. При ошибке активации возвращает предыдущий бинарь.

На Ubuntu-сервере установщик ставит недостающие пакеты сам, если панель
запущена с правами root. Вручную это выглядит так:

```bash
sudo apt update
sudo apt install -y git golang-go
```

## systemd

Для каждого профиля создаётся unit:

```text
/etc/systemd/system/openflux-<id>.service
```

Команда запуска:

```bash
/usr/local/bin/openflux --role=exit --mode=<l4|l3> --transport=<yandex|boards|vyandex> --url=<secret> --codec=batched --debug=<0..3>
```

Если указан файл ключа, добавляется `--encryption-key-file=<path>`.

## Panel API

Все пути ниже требуют авторизацию панели:

- `GET /panel/api/openflux/list`
- `GET /panel/api/openflux/install-info`
- `GET /panel/api/openflux/mobile-connections`
- `GET /panel/api/openflux/version`
- `GET /panel/api/openflux/status/:id`
- `GET /panel/api/openflux/unit/:id`
- `POST /panel/api/openflux/install`
- `POST /panel/api/openflux/add`
- `POST /panel/api/openflux/update/:id`
- `POST /panel/api/openflux/delete/:id`
- `POST /panel/api/openflux/apply/:id`
- `POST /panel/api/openflux/start/:id`
- `POST /panel/api/openflux/stop/:id`
- `POST /panel/api/openflux/restart/:id`

Пример установки из GitHub:

```json
{
  "ref": "HEAD"
}
```

## Mobile API

Для мобильного клиента добавлены пути:

- `GET /api/mobile/subscriptions`
- `GET /api/mobile/subscriptions/:subid`

Ответ содержит включённые OpenFlux-профили:

```json
{
  "success": true,
  "connections": [
    {
      "name": "nl2 Yandex Docs",
      "server": "nl2",
      "module": "openflux_yandex_docs_cursor_ws",
      "protocol": "openflux",
      "transport": "yandex",
      "carrier": "yandex_docs",
      "module_config": "{\"codec\":\"batched\",\"debug\":1,\"mode\":\"l4\",\"transport\":\"yandex\",\"url\":\"https://disk.yandex.ru/i/...\"}",
      "enabled": true
    }
  ]
}
```

Маппинг carrier:

- `yandex` -> `yandex_docs`
- `boards` -> `cursor_ws`
- `vyandex` -> `volga_ws`

`module_config` содержит рабочий URL намеренно: мобильному модулю он нужен для подключения. Не публикуйте этот endpoint без штатной защиты подписок.

## Ограничения

- Учёт трафика OpenFlux не добавлен в статистику Xray: у OpenFlux должен появиться стабильный источник counters/API.
- Синтетический probe профиля пока не выделен отдельно; рабочее состояние проверяется через systemd и journalctl.
- Сборка из GitHub требует сетевой доступ сервера к GitHub и Go-модулям.
