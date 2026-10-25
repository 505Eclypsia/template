# TripGo — репозиторий для лабораторных работ

## Лабораторная работа 1: Trip Service

Микросервис управления поездками на Go (PostgreSQL, chi, pgxpool, squirrel).

### Инструкция по запуску

1. Запустить локальный кластер и окружение:
   ```bash
   tripgoctl cluster start
   tripgoctl environment start
   ```
2. Применить миграции базы данных:
   ```bash
   make migrate
   ```
3. Запустить сервис:
   ```bash
   make run
   ```

### Переменные окружения (.env)

| Переменная | Описание | Пример |
|---|---|---|
| `HTTP_ADDR` | Адрес и порт HTTP-сервера | `:8080` |
| `LOG_LEVEL` | Уровень логирования | `info` |
| `SHUTDOWN_TIMEOUT` | Таймаут плавного выключения | `10s` |
| `DATABASE_URL` | Строка подключения к PostgreSQL | `postgres://...` |
| `DATABASE_MAX_CONNS` | Максимум соединений в пуле | `10` |
| `DATABASE_MIN_CONNS` | Минимум соединений в пуле | `2` |
| `DATABASE_MAX_CONN_LIFETIME` | Время жизни соединения | `30m` |
| `DATABASE_CONNECT_TIMEOUT` | Таймаут на подключение к БД | `5s` |
| `DATABASE_QUERY_TIMEOUT` | Таймаут на выполнение запроса | `3s` |

### Решения

1. **Уровень изоляции транзакций:**
   Выбран `Read Committed` (по умолчанию в PostgreSQL). Он обеспечивает максимальную производительность без избыточных блокировок. Защита от создания дубликатов поездок для активного водителя обеспечивается на уровне базы через уникальный частичный индекс, а защита от гонки при завершении поездки — атомарным `UPDATE ... WHERE status = 'active'`.

2. **Менеджер транзакций (`TxManager`):**
   Реализован через контекст (`context.WithValue`). Метод `Do` открывает транзакцию, обогащает контекст и передает его в функцию. Репозиторий достает исполнителя через `ctx.Value`. При успехе вызывается `Commit`, при ошибке или панике — `Rollback`. Вложенный `Do` проверяет наличие транзакции в контексте и переиспользует её.

3. **Запрет двух активных поездок на одного водителя:**
   Обеспечен частичным уникальным индексом в схеме PostgreSQL:
   ```sql
   CREATE UNIQUE INDEX idx_trips_driver_active ON trips (driver_id) WHERE status = 'active';
   ```
   Ошибка `23505` (`unique_violation`) перехватывается в репозитории и мапится в доменную ошибку `ErrDriverBusy` (отдаётся клиенту как `409 driver_busy`).

4. **Идемпотентность:**
   Реализована через отдельную таблицу `trip_idempotency_keys` с хранением `Idempotency-Key` и `SHA-256` хэша тела запроса. Повторный запрос с тем же ключом и тем же телом возвращает `200 OK` и ранее созданную поездку. Тот же ключ с измененным телом возвращает `409 idempotency_conflict`.

5. **Многостадийная сборка:**
   Итоговый Docker-образ собран на базе `gcr.io/distroless/static-debian12` и запускается от непривилегированного пользователя `nonroot`.
   * **Размер итогового образа:** `22.8 MB`.
   #### 1. Сборка образа
   ```bash
   docker build -t trip-service:latest -f deploy/Dockerfile .
   ```

   #### 2. Запуск контейнера рядом с локальным PostgreSQL
   ```bash
   docker run --rm -d \
   --name trip-service-app \
   -p 8080:8080 \
   --env-file .env \
   -e DATABASE_URL="postgres://tripgo:tripgo@host.docker.internal:21032/tripgo?sslmode=disable" \
   trip-service:latest
   ```