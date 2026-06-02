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

### Google OAuth без еженедельной повторной авторизации

Сервер хранит Google OAuth token в Postgres и автоматически обновляет access
token через refresh token. Если авторизация слетает примерно раз в неделю,
проверь Google Cloud: для External OAuth consent screen в статусе `Testing`
Google выдает refresh token с истечением через 7 дней.

Чтобы авторизация жила дольше:

1. Открой Google Cloud Console -> `APIs & Services` -> `OAuth consent screen`.
2. Переведи publishing status из `Testing` в `In production`.
3. Убедись, что OAuth client содержит redirect URI:

   ```text
   http://localhost:8080/oauth/google/callback
   ```

4. Авторизуй Google через `http://localhost:8080` в браузере на той же машине,
   где запущен сервер. Google OAuth не принимает private LAN IP вроде
   `http://192.168.x.x:8080/oauth/google/callback` как redirect URI.

5. После изменения статуса авторизуй Google в UI еще один раз, чтобы получить
   новый refresh token уже не из Testing-режима.

Не нажимай `Отключить`, не удаляй `data/` и не сбрасывай таблицу
`google_tokens`, если не хочешь принудительно сбросить авторизацию.

Для Windows-машины с кассой используй подробный чеклист:
`WINDOWS_DOCKER_CHECKLIST.md`.

## Telegram fax bot

Сервер может работать как персональный fax-принтер для Telegram:
разрешенный человек пишет в личный чат, Telegram передает `business_message`
или обычный `message` подключенному боту, а сервер печатает это сообщение как
нефискальный чек.

В v1 бот:
- читает Telegram Business updates и обычные личные сообщения боту;
- игнорирует групповые чаты и команды вроде `/start`;
- печатает сразу, без подтверждения;
- не отвечает собеседнику в Telegram;
- для обычной лички бота печатает от любого человека, который написал боту;
- для Telegram Business проверяет разрешенного Business-владельца, а sender
  allowlist можно оставить пустым и доверять выбранным чатам в Telegram.

Настройка:

1. Создай бота через `@BotFather`.
2. Включи для него Business/Secretary mode в `@BotFather`, если этот пункт
   доступен в твоей версии Telegram.
3. В Telegram аккаунте с Premium открой `Settings -> Telegram Business ->
   Chatbots` и подключи бота к личным чатам, которые можно передавать боту.
4. До запуска сервера получи Telegram user IDs. Самый простой официальный
   способ: попросить каждого человека написать любое сообщение боту и открыть:

   ```text
   https://api.telegram.org/bot<TELEGRAM_FAX_BOT_TOKEN>/getUpdates
   ```

   В ответе нужны поля `message.from.id`. Твой ID идет в
   `TELEGRAM_FAX_OWNER_IDS`.

   `TELEGRAM_FAX_ALLOWED_SENDER_IDS` можно оставить пустым для Telegram
   Business: тогда сервер печатает всех отправителей, чьи чаты ты разрешил в
   Telegram Business. Обычная личка бота открыта для любого человека, который
   написал боту, и этот список ее не ограничивает.
5. Создай локальный файл `.env` рядом с `docker-compose.yml`. Можно начать с
   примера:

   ```bash
   cp .env.example .env
   ```

   Заполни `.env` реальными значениями:

   ```dotenv
   TELEGRAM_FAX_BOT_TOKEN=123456:replace-with-bot-token
   TELEGRAM_FAX_OWNER_IDS=111111111
   TELEGRAM_FAX_ALLOWED_SENDER_IDS=
   TELEGRAM_FAX_POLL_TIMEOUT_SECONDS=25
   ```

   `.env` игнорируется git-ом. Коммитить можно только `.env.example`.

6. Пересоздай контейнер. Docker Compose автоматически подхватит `.env`:

   ```bash
   docker compose up -d --force-recreate atol-server
   ```

В логах должно появиться `Telegram fax bot enabled`. Последний
обработанный Telegram update хранится в Postgres, поэтому после рестарта старые
сообщения не печатаются повторно.

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

Перед рефакторингом серверного кода используй quality gate из
`docs/server-quality-gate.md`: `go test ./...`, `go vet ./...` и
`go test -cover ./...`.
