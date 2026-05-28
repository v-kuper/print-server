# ATOL Go Server

Локальный сервер печатает нефискальные чеки на ATOL по TCP/IP:
тестовый чек, дневной чек с погодой/курсами/новостями/Google Calendar,
изображение из pixel buffer и разовый форматированный текст.

По умолчанию Docker-образ использует plain-драйвер из
`driver/linux-amd64`. Сейчас туда положен ATOL Driver 10.10.7.0,
который не требует UEMA.

Если нужно заменить драйвер, положи туда Linux x64 runtime-файлы старой
plain-версии:

```text
driver/linux-amd64/libfptr10.so
```

Если в старом дистрибутиве рядом лежат `libusb-1.0.so.0`, `libudev.so.0`
или другие `.so*`, скопируй их туда же.

UEMA target для 10.10.8.0 оставлен в Dockerfile как `runtime-uem`, но для
нашего локального сервера он сейчас не основной: 10.10.8.0 требует UEMA и
может упираться в регистрацию агента/облачную часть ATOL.

## Запуск

Из корня этого репозитория:

```bash
docker compose build --no-cache atol-server
docker compose up -d --force-recreate atol-server
```

После запуска открой:

```text
http://localhost:8080
```

Основное хранилище — Postgres из `docker-compose.yml`. Старые файлы
`data/settings.json`, `data/image-editor/*` и `data/google/token.json`
импортируются в БД один раз при первом запуске и остаются как backup.
Google `credentials.json` по-прежнему нужно положить в `data/google/credentials.json`.

Для Windows-машины с кассой используй подробный чеклист:
`WINDOWS_DOCKER_CHECKLIST.md`.

## Проверка

1. Введи IP кассы, например `192.168.0.118`.
2. Введи порт, например `5555`.
3. Нажми `Проверить связь`.
4. Нажми `Напечатать тестовый чек`.
5. Для дневного чека проверь город/координаты, TON-настройки, RSS-ленты,
   Google Calendar и нажми `Напечатать чек`.
6. Для разовой печати используй блоки `Изображение на чек` и `Текст на чек`.

Погода берется из Open-Meteo без API key. По умолчанию выставлен Гомель:
`52.4345, 30.9754`.

Курсы:
- TON/USD берется из CoinGecko Simple Price API.
- USD/BYN берется из API НБРБ.

RSS по умолчанию:
- BBC Russian;
- Reuters;
- Economist;
- Hacker News.

## API

```text
GET  /healthz
POST /api/settings/printer
POST /api/settings/weather
POST /api/settings/finance
POST /api/settings/news
POST /api/settings/receipt-snapshot
POST /api/printer/check
POST /api/printer/fonts
POST /api/receipt/preview
POST /api/print/test
POST /api/print/text
POST /api/print/weather
GET  /snapshots/{id}
GET  /api/image-editor/state
POST /api/image-editor/save
GET  /api/image-editor/preview
POST /api/image-editor/print
DELETE /api/image-editor/image
```

## Локальные тесты

```bash
GOCACHE="$(pwd)/.gocache" go test ./...
```
