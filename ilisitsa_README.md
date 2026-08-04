# vanilla_trader_bot

**Vanilla** — торговый робот на Go, который выставляет **одну лимитную заявку** на **один инструмент**, ждёт её финального статуса и печатает результат. Транспорт — RabbitMQ (AMQP); поддерживаются инструменты Сбер (`SBER@TQBR`) и Юань (`CNYRUB_TOM@CETS`).

---

## Установка и запуск

**Требования:** Go 1.21+

```sh
git clone <repo>
cd vanilla_trader_bot
go run ./cmd -t TICKER -m MARKET (--qty QTY | --volume VOL) (--price PRICE | --slippage PCT | --best_quote) [--config DIR]
```

Или собрать бинарь:

```sh
go build -o vanilla ./cmd
./vanilla -t SBER -m TQBR --qty 10 --price 303.15
```

---

## Параметры командной строки

| Флаг | Алиас | Тип | Описание |
|------|-------|-----|----------|
| `--ticker` | `-t` | string | Тикер инструмента, напр. `SBER` |
| `--market` | `-m` | string | Код класса, напр. `TQBR` |
| `--qty` | `-q` | int | Количество лотов (+ покупка, − продажа) |
| `--volume` | `-v` | decimal | Объём в рублях (+ покупка, − продажа) |
| `--price` | `-p` | decimal | Фиксированная лимитная цена |
| `--slippage` | `-s` | decimal | Проскальзывание в % от последней цены |
| `--best_quote` | `-b` | bool | Лимит = лучший ask/bid из стакана |
| `--config` | `-c` | path | Папка с `config.json`, `queues.json`, `securities/` (по умолчанию `./config/data/`) |

Обязательно передать ровно один флаг объёма (`--qty` или `--volume`) и ровно один флаг цены (`--price`, `--slippage` или `--best_quote`). При конфликте приоритет: `qty` > `volume`, `price` > `slippage` > `best_quote`.

### Примеры

```sh
# Купить 10 лотов Сбера по фиксированной цене
go run ./cmd -t SBER -m TQBR --qty 10 --price 303.15

# Продать на 50 000 руб с проскальзыванием 0.1%
go run ./cmd -t SBER -m TQBR --volume -50000 --slippage 0.1

# Купить юань по лучшему ask из стакана
go run ./cmd -t CNYRUB_TOM -m CETS --qty 1000 --best_quote
```

---

## Конфигурация

Директория конфига (по умолчанию `./config/data/`) содержит три части.

**`config.json`** — подключение к RabbitMQ:

```json
{
  "url": "amqp://user:pass@host:5672/",
  "awaitTimeout": "5s"
}
```

`awaitTimeout` — максимальное ожидание рыночных данных перед отправкой заявки (формат `time.Duration`).

**`queues.json`** — топология exchange/очередей (publisher и четыре consumer: `statuses`, `trades`, `orderbooks`, `ticks`).

**`securities/*.json`** — параметры инструментов: размер лота (`lotSize`) и минимальный шаг цены (`minStep`).

---

## Архитектура

```
cmd/
  main.go                     точка входа
internal/
  app/
    app.go                    CLI-приложение (urfave/cli)
    query/query.go            разбор аргументов → структура Query
  ordering/
    ordering.go               PlaceOrder: подключение к AMQP, публикация заявки, сборка отчёта
    pricing.go                computePriceQty: расчёт лимитной цены и количества лотов
    awaiting/
      broker.go               инициализация AMQP-каналов (1 publisher + 4 consumers)
      handlers.go             обработчики входящих сообщений (статусы, трейды)
      messages.go             JSON-структуры входящих/исходящих сообщений
      monitor.go              Monitor: ожидание финального статуса / исполнения
      refreshing.go           RefreshingValue[T] — обновляемое значение с подпиской, MPSC
config/
  config.go                   загрузка config.json, queues.json, securities/
  structures.go               типы Config, Security, SecurityData, Consumer
  id.go                       генерация уникального RunID (UUID)
```

Каждый consumer работает в отдельной горутине через общий примитив `RefreshingValue[T]`: значение атомарно обновляется по каждому сообщению, а ожидающая горутина получают уведомление через канал.

---

## Алгоритм работы

```
1. Разобрать аргументы CLI → Query.
2. Загрузить конфиг (RabbitMQ URL, параметры инструмента, топология очередей).
3. Подключиться к RabbitMQ, открыть 5 каналов.
4. Объявить exclusive auto-delete очереди (max 1000 msg, drop-head) и забиндить:
     sandbox.order.status   → Statuses
     sandbox.trades         → Trades
     marketdata.orderbooks.alor → Orderbooks
     marketdata.ticks.alor  → Ticks
5. Запустить горутины обработки Orderbooks и Ticks.
6. Рассчитать лимитную цену.
7. Рассчитать количество лотов:
     --qty    → взять значение напрямую
     --volume → floor(volume / (price × lotSize))
8. Запустить Monitor (слушает Statuses + Trades).
9. Опубликовать JSON-заявку в sandbox.orders (routing key locko.place).
10. Monitor.Wait():
      - копит трейды по orderId, обновляет исполненные лоты и оборот
      - завершается при status=Filled/Cancelled или когда лоты из трейдов ≥ qty
      - reconcile: синхронизирует данные статуса и трейдов (если заявка cancelled, мы ждем те трейды, которые до нас не дошли, чтобы правильно все рассчитать)
11. Вывести результат в консоль.
12. Соединение закрывается (defer), очереди удаляются автоматически (auto-delete).
```

---

## Формат вывода

```
Ордер 51814779260 (SBER@TQBR, BUY) [Filled]: исполнено 10 лотов на 30310.00 RUB, средняя цена 303.10000. Количество сделок: 2
```

Если ордер отменён и ничего не исполнилось:

```
Ордер 51814779260 (SBER@TQBR, BUY) [Cancelled]: исполнено 0 лотов на 0.00 RUB
```

---

## Зависимости

| Библиотека | Назначение |
|------------|------------|
| `github.com/rabbitmq/amqp091-go` | AMQP-клиент (RabbitMQ) |
| `github.com/shopspring/decimal` | арифметика с фиксированной точкой для цен |
| `github.com/urfave/cli/v2` | разбор аргументов командной строки |
| `github.com/google/uuid` | генерация уникального RunID |
