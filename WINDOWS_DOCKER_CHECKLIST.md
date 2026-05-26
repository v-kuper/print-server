# Windows Docker checklist

Этот файл лежит внутри папки `server`. На Windows открой PowerShell прямо в
папке `server` и выполняй команды сверху вниз.

## 1. Проверить, что ты внутри server

```powershell
Test-Path .\docker-compose.yml
```

Должно вывести:

```text
True
```

## 2. Проверить Docker

Открой Docker Desktop и дождись запуска. Затем:

```powershell
docker version
docker compose version
docker info
```

Если Docker Desktop спросит режим контейнеров, нужен `Linux containers`.

## 3. Проверить ATOL driver

```powershell
Test-Path .\driver\linux-amd64\libfptr10.so
```

Должно вывести:

```text
True
```

Если `False`, положи Linux x64 runtime-файлы ATOL Driver в:

```text
driver\linux-amd64\
```

Минимально нужен:

```text
driver\linux-amd64\libfptr10.so
```

Если рядом есть `libusb-1.0.so.0`, `libudev.so.0` или другие `.so*`, положи
их туда же.

## 4. Положить Google credentials

```powershell
New-Item -ItemType Directory -Force .\data\google | Out-Null
```

Положи OAuth-файл Google сюда:

```text
data\google\credentials.json
```

Если файл лежит в Downloads:

```powershell
Copy-Item "$env:USERPROFILE\Downloads\credentials.json" .\data\google\credentials.json -Force
Test-Path .\data\google\credentials.json
```

Должно вывести:

```text
True
```

В Google Cloud OAuth client должен быть redirect URI:

```text
http://localhost:8080/oauth/google/callback
```

Важно: `data\` локальная папка, её нельзя отправлять в публичный репозиторий.

## 5. Собрать Docker image без cache

```powershell
docker compose build --no-cache atol-server
```

## 6. Запустить контейнер

```powershell
docker compose up -d --force-recreate atol-server
```

## 7. Проверить статус контейнера

```powershell
docker compose ps atol-server
```

В колонке `STATUS` должно быть `Up`.

## 8. Проверить, что сервер отвечает

```powershell
Invoke-WebRequest http://localhost:8080/ -UseBasicParsing
```

Если ответ пришел без ошибки:

```powershell
Start-Process http://localhost:8080/
```

## 9. Настроить в UI

В браузере:

1. В блоке `Касса` укажи IP кассы и порт.
2. Нажми `Проверить связь`.
3. Нажми `Тестовый чек`.
4. В блоке `Google` нажми `Авторизовать` и пройди OAuth.
5. В блоке `Состав чека` включи нужные секции.
6. Нажми `Показать превью`.
7. Нажми большую кнопку `Напечатать чек`.

## 10. Команды на каждый день

Запустить:

```powershell
docker compose up -d atol-server
```

Остановить:

```powershell
docker compose down
```

Перезапустить:

```powershell
docker compose restart atol-server
```

Посмотреть логи:

```powershell
docker compose logs --tail=120 atol-server
```

Следить за логами:

```powershell
docker compose logs -f --tail=80 atol-server
```

Остановить просмотр логов: `Ctrl+C`.

## 11. Если меняешь weather icons

Готовые для кассы иконки лежат здесь:

```text
assets\weather-icons\print\
```

Подготовить иконки заново и пересобрать контейнер:

```powershell
go run .\cmd\prepare-icons -source assets\weather-icons\print -target assets\weather-icons\print -size 96
docker compose build --no-cache atol-server
docker compose up -d --force-recreate atol-server
```

Если кладешь исходники в `assets\weather-icons\source\`:

```powershell
go run .\cmd\prepare-icons
docker compose build --no-cache atol-server
docker compose up -d --force-recreate atol-server
```

## 12. Частые проблемы

### Порт 8080 занят

```powershell
netstat -ano | findstr :8080
```

Остановить контейнер:

```powershell
docker compose down
```

### Google пишет redirect_uri_mismatch

Проверь, что в Google Cloud OAuth client добавлен redirect URI:

```text
http://localhost:8080/oauth/google/callback
```

После изменения в Google Cloud заново нажми `Авторизовать` в UI.

### print picture failed: Размер картинки слишком большой (156)

```powershell
go run .\cmd\prepare-icons -source assets\weather-icons\print -target assets\weather-icons\print -size 96
docker compose build --no-cache atol-server
docker compose up -d --force-recreate atol-server
```

### Контейнер не стартует

```powershell
docker compose ps atol-server
docker compose logs --tail=120 atol-server
```
