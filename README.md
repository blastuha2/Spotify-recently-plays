# Spotify History Export

Локальное CLI-приложение на Go с двумя режимами выгрузки истории Spotify.

## Режимы

### `official` — официальный Spotify Web API

Использует `GET /v1/me/player/recently-played`, сохраняет точное время `played_at`, контекст и остальные исходные данные. Spotify часто возвращает только последние 50 событий, даже если в ответе присутствует `next`.

Файлы по умолчанию:

```text
spotify_history.csv
spotify_history.json
```

### `partner` — внутренний API Spotify Web Player

Использует `https://api-partner.spotify.com/pathfinder/v2/query`, запрашивает по 100 элементов и проходит страницы через `offset`/`nextOffset` до указанной даты или конца доступного списка. Обычно глубина ограничена Spotify примерно 90 днями.

Из общего списка экспортируются только элементы типа `TRACK`; пустые элементы, альбомы и плейлисты пропускаются. Повторные появления одного URI сохраняются как отдельные события, поскольку они могут соответствовать повторным прослушиваниям.

Внутренний API возвращает только календарную дату `addedAt`, без точного времени прослушивания. Поэтому partner-файлы содержат `played_date`, но не выдуманное `played_at`.

Файлы по умолчанию:

```text
spotify_partner_history.csv
spotify_partner_history.json
```

Этот endpoint официально не поддерживается Spotify. Persisted-query hash, заголовки и схема ответа могут измениться.

## Требования

- Go 1.22 или новее;
- Windows PowerShell;
- временные токены Spotify для выбранного режима.

Никогда не добавляйте реальные токены в исходный код, README, CSV, JSON или Git.

## Запуск official

Получите access token со scope `user-read-recently-played` через Spotify Developer / Try it. Передавайте только значение токена, без слова `Bearer`:

```powershell
Set-Location "C:\Users\Boris\Desktop\Others\Go\SpotifyScript"
$env:SPOTIFY_TOKEN="BQA..."
go run . -mode official -from 2026-04-11
```

Для обратной совместимости `official` является режимом по умолчанию:

```powershell
go run . -from 2026-04-11
```

## Запуск partner

Откройте `https://open.spotify.com`, войдите в аккаунт и откройте DevTools (`F12`):

1. На вкладке **Network** отфильтруйте запросы по `pathfinder`.
2. Выберите запрос `query`, у которого в **Payload** указано `"operationName":"recents"`.
3. На вкладке **Headers** скопируйте значения заголовков `authorization`, `client-token` и `spotify-app-version`.
4. Не публикуйте эти значения и не вставляйте их в файлы проекта.

После клонирования создайте локальный `.env` из безопасного шаблона:

```powershell
Copy-Item .env.example .env
```

Откройте `.env` и вставьте значения после знака `=`. В `SPOTIFY_PARTNER_TOKEN` слово `Bearer` добавлять не нужно:

```dotenv
SPOTIFY_PARTNER_TOKEN=BQC...
SPOTIFY_CLIENT_TOKEN=AA...
SPOTIFY_APP_VERSION=1.2...
SPOTIFY_RECENTS_HASH=
```

После этого запускайте без команд `$env:`:

```powershell
go run . -mode partner -from 2026-04-11
```

Если `persistedQuery` изменился и Spotify сообщает GraphQL-ошибку, скопируйте новый `sha256Hash` из Payload:

```dotenv
SPOTIFY_RECENTS_HASH=новый_sha256Hash
```

```powershell
go run . -mode partner -from 2026-04-11
```

Если `-from` не указан, в обоих режимах используется текущая дата минус четыре календарных месяца. Для partner-режима это обычно позволяет получить весь доступный период, поскольку четыре месяца длиннее ожидаемого 90-дневного окна.

## Свои пути результатов

```powershell
go run . -mode partner -from 2026-04-11 `
  -csv "C:\Users\Boris\Desktop\partner.csv" `
  -json "C:\Users\Boris\Desktop\partner.json"
```

CSV записывается в UTF-8 с BOM для корректного открытия кириллицы в Excel. JSON сохраняет список исполнителей отдельным массивом.

## Сборка

```powershell
go build -o spotify-history.exe
```

Запуск собранной программы:

```powershell
.\spotify-history.exe -mode official -from 2026-04-11
.\spotify-history.exe -mode partner -from 2026-04-11
```

## Проверка

```powershell
go test ./...
go vet ./...
```

Токены читаются из `.env` или переменных окружения, не записываются в результаты и не выводятся в лог. Значения из PowerShell имеют приоритет над `.env`. Файл `.env` добавлен в `.gitignore`. Токены Web Player временные: при `401`/`403` получите свежие значения из нового запроса браузера.
