# ATOL Go Server

Первый серверный MVP печатает нефискальные чеки на ATOL по TCP/IP:
тестовый чек и дневной чек с погодой, курсами валют и RSS-новостями.

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
docker compose build --no-cache
docker compose up -d
```

После запуска открой:

```text
http://localhost:8080
```

Настройки сохраняются в `data/settings.json`.

## Проверка

1. Введи IP кассы, например `192.168.0.118`.
2. Введи порт, например `5555`.
3. Нажми `Проверить связь`.
4. Нажми `Напечатать тестовый чек`.
5. Для дневного чека проверь город/координаты, TON-настройки, RSS-ленты и нажми `Напечатать чек`.

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
POST /api/printer/check
POST /api/print/test
POST /api/print/weather
```

## Локальные тесты

```bash
GOCACHE="$(pwd)/.gocache" go test ./...
```
