package awaiting

import (
	"context"
	"encoding/json"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/shopspring/decimal"
)

type StatusHandler struct {
	Ctx               context.Context
	OrderName         string
	orderIDSeen       bool
	OrderID           int64
	OrderIDReady      chan struct{}
	StatusLots        *RefreshingValue[int64]
	TerminationStatus chan string
}

func (h *StatusHandler) Handle(d amqp.Delivery) {
	var status OrderStatus
	if json.Unmarshal(d.Body, &status) == nil && status.StrategyName == h.OrderName {
		if !h.orderIDSeen {
			h.orderIDSeen = true
			h.OrderID = status.OrderID
			close(h.OrderIDReady)
		}
		h.StatusLots.Store(status.RemainingQty.IntPart())
		if status.Status != "Submitted" {
			select {
			case h.TerminationStatus <- status.Status:
			case <-h.Ctx.Done():
			}
		}
	}
}

type OrderState struct {
	LotsProcessed int64
	LotsRemaining int64

	TotalCashflow decimal.Decimal
	TotalTrades   int64
}

// WithTrade returns a copy of the state with one fill applied.
func (o OrderState) WithTrade(qty int64, volume decimal.Decimal) OrderState {
	o.LotsProcessed += qty
	o.LotsRemaining -= qty
	o.TotalTrades++

	o.TotalCashflow = o.TotalCashflow.Add(volume)
	return o
}

type TradeHandler struct {
	Ctx          context.Context
	OrderIDReady chan struct{}
	OrderID      *int64
	State        *RefreshingValue[OrderState]
}

func (h *TradeHandler) Handle(d amqp.Delivery) {
	var trade Trade
	select {
	case <-h.OrderIDReady:
	case <-h.Ctx.Done():
		return
	}
	if json.Unmarshal(d.Body, &trade) == nil && trade.OrderID == *h.OrderID {
		h.State.Store(h.State.value.WithTrade(trade.Qty.Abs().IntPart(), trade.Volume))
	}
}

type OrderBookHandler struct {
	Ticker, Market string
	Book           *RefreshingValue[OrderBook]
}

func (h *OrderBookHandler) Handle(d amqp.Delivery) {
	var temp OrderBook
	if json.Unmarshal(d.Body, &temp) == nil && temp.Ticker == h.Ticker && temp.Market == h.Market {
		h.Book.Store(temp)
	}
}

type TickHandler struct {
	Ticker, Market string
	Tick           *RefreshingValue[Tick]
}

func (h *TickHandler) Handle(d amqp.Delivery) {
	var temp Tick
	if json.Unmarshal(d.Body, &temp) == nil && temp.SecurityID == h.Ticker && temp.SecurityExchange == h.Market {
		h.Tick.Store(temp)
	}
}
