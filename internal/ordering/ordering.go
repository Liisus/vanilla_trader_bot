package ordering

import (
	"context"
	"encoding/json"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"

	"vanilla/config"
	"vanilla/internal/app/query"
	"vanilla/internal/ordering/awaiting"
)

func abs(x int64) int64 {
	if x > 0 {
		return x
	}
	return -x
}

func PlaceOrder(parent context.Context, query *query.Query) (Report, error) {
	conn, err := amqp.Dial(query.URL)
	if err != nil {
		return Report{}, fmt.Errorf("dial rabbitmq: %w", err)
	}
	defer conn.Close()

	orderName := fmt.Sprintf("ilisitsa_%s@%s_%s", query.Ticker, query.Market, config.RunID)

	ctx, cancel := context.WithCancel(parent)
	defer cancel()

	b, err := awaiting.SetupBroker(conn, query)
	if err != nil {
		return Report{}, err
	}

	obh := awaiting.OrderBookHandler{
		Ticker: query.Ticker,
		Market: query.Market,
		Book:   awaiting.NewRefreshingValue[awaiting.OrderBook]()}
	th := awaiting.TickHandler{
		Ticker: query.Ticker,
		Market: query.Market,
		Tick:   awaiting.NewRefreshingValue[awaiting.Tick]()}

	go awaiting.Refresher(ctx, b.Orderbooks, obh.Handle)
	go awaiting.Refresher(ctx, b.Ticks, th.Handle)

	price, qty, err := computePriceQty(ctx, *query, obh, th)
	if err != nil {
		return Report{}, err
	}

	monitor := awaiting.StartMonitor(ctx, &b, orderName, abs(qty))

	err = publishOrder(parent, b.PublishOrder, order{
		name:   orderName,
		price:  price,
		qty:    qty,
		ticker: query.Ticker,
		market: query.Market,
	})
	if err != nil {
		return Report{}, err
	}

	state, finalStatus := monitor.Wait()

	return Report{
		OrderID:         monitor.OrderID(),
		Ticker:          query.Ticker,
		Market:          query.Market,
		Side:            sideLabel(qty),
		ProcessedLots:   state.LotsProcessed,
		ProcessedTrades: state.TotalTrades,
		Volume:          state.TotalCashflow,
		LotSize:         query.LotSize,
		Status:          finalStatus,
	}, nil
}

func sideLabel(qty int64) string {
	if qty < 0 {
		return "SELL"
	}
	return "BUY"
}

func publishOrder(ctx context.Context, publish func(context.Context, []byte) error, params order) error {
	order := awaiting.OrderRequest{
		OrderStrategyID: params.name,
		OrderSecurityID: fmt.Sprintf("%s@%s", params.ticker, params.market),
		OrderType:       "LMT",
		OrderPrice:      params.price,
		OrderQty:        params.qty,
		OrderPortfolio:  "Gurren-dan",
		OrderClientCode: "Kamina",
	}

	body, err := json.Marshal(order)
	if err != nil {
		return fmt.Errorf("marshal order: %w", err)
	}

	return publish(ctx, body)
}
