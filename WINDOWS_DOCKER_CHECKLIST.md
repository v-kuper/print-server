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

## 9. Открыть доступ из локальной сети

Этот шаг нужен, чтобы открыть интерфейс с Mac, телефона или другого устройства
в той же локальной сети:

```text
http://<IP Windows-машины>:8080/
```

Команды изменения сетевого профиля и firewall-правил выполняй в PowerShell от
имени администратора.

Сначала проверь, что Windows считает текущую сеть доверенной:

```powershell
Get-NetConnectionProfile
```

В колонке `NetworkCategory` должно быть `Private`. Если там `Public`, поменяй
категорию для нужного интерфейса. Надежнее использовать `InterfaceIndex` из
вывода команды:

```powershell
Set-NetConnectionProfile -InterfaceIndex 17 -NetworkCategory Private
Get-NetConnectionProfile -InterfaceIndex 17
```

Если у твоего интерфейса другой `InterfaceIndex`, подставь его вместо `17`.
Например, для вывода:

```text
InterfaceAlias  : Беспроводная сеть
InterfaceIndex  : 17
NetworkCategory : Public
```

нужно выполнить именно команду с `-InterfaceIndex 17`.

Можно использовать и имя интерфейса:

```powershell
Set-NetConnectionProfile -InterfaceAlias "Wi-Fi" -NetworkCategory Private
```

Если интерфейс называется иначе, возьми имя из колонки `InterfaceAlias`.

Открой входящий TCP-порт `8080` в Windows Firewall только для приватной сети:

```powershell
if (-not (Get-NetFirewallRule -DisplayName "ATOL Go Server 8080 LAN" -ErrorAction SilentlyContinue)) {
    New-NetFirewallRule `
        -DisplayName "ATOL Go Server 8080 LAN" `
        -Direction Inbound `
        -Action Allow `
        -Protocol TCP `
        -LocalPort 8080 `
        -Profile Private
}
```

Узнай IPv4-адрес Windows-машины в локальной сети:

```powershell
Get-NetIPAddress -AddressFamily IPv4 | Where-Object {
    $_.IPAddress -notlike "169.254.*" -and
    $_.IPAddress -ne "127.0.0.1"
} | Select-Object InterfaceAlias,IPAddress
```

С Mac или телефона, подключенного к той же Wi-Fi/LAN сети, открой:

```text
http://<IP Windows-машины>:8080/
```

Например:

```text
http://192.168.0.25:8080/
```

Чтобы адрес не менялся после перезагрузки, закрепи этот IP за Windows-машиной
в настройках DHCP/роутера.

Если `http://localhost:8080/` работает на Windows, но с Mac/телефона страница
не открывается, проверь:

- Windows и второе устройство подключены к одной сети;
- `NetworkCategory` на Windows равен `Private`;
- firewall-правило `ATOL Go Server 8080 LAN` создано;
- в адресе указан IPv4 Windows-машины, а не IP кассы;
- на роутере не включена изоляция Wi-Fi клиентов.

Если проходишь Google-авторизацию не на Windows-машине, а с Mac или телефона,
добавь в Google Cloud OAuth client redirect URI с LAN-адресом:

```text
http://<IP Windows-машины>:8080/oauth/google/callback
```

`http://localhost:8080/oauth/google/callback` продолжает работать только при
авторизации прямо на Windows-машине.

## 10. Настроить в UI

В браузере:

1. В блоке `Касса` укажи IP кассы и порт.
2. Нажми `Проверить связь`.
3. Нажми `Тестовый чек`.
4. В блоке `Google` нажми `Авторизовать` и пройди OAuth.
5. В блоке `Состав чека` включи нужные секции.
6. Нажми `Показать превью`.
7. Нажми большую кнопку `Напечатать чек`.

## 11. Команды на каждый день

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

## 12. Если меняешь weather icons

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

## 13. Частые проблемы

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
