package test

import (
	"context"
	"testing"
	"time"

	"vanilla/internal/ordering/awaiting"

	"github.com/shopspring/decimal"
)

// ---- OrderBookHandler ----

func TestOrderBookHandler_MatchingMessage(t *testing.T) {
	h := awaiting.OrderBookHandler{
		Ticker: "SBER", Market: "TQBR",
		Book: awaiting.NewRefreshingValue[awaiting.OrderBook](),
	}

	h.Handle(delivery(`{
		"ticker":"SBER","market":"TQBR",
		"bid_prices":["303.10","303.09"],
		"ask_prices":["303.15","303.16"]
	}`))

	book := h.Book.Get()
	if len(book.AskPrices) == 0 || !book.AskPrices[0].Equal(decimal.RequireFromString("303.15")) {
		t.Fatalf("ask[0] not stored: %+v", book.AskPrices)
	}
	if !book.BidPrices[0].Equal(decimal.RequireFromString("303.10")) {
		t.Errorf("bid[0] = %s, want 303.10", book.BidPrices[0])
	}
}

func TestOrderBookHandler_FiltersForeignTicker(t *testing.T) {
	h := awaiting.OrderBookHandler{
		Ticker: "SBER", Market: "TQBR",
		Book: awaiting.NewRefreshingValue[awaiting.OrderBook](),
	}
	// Wrong ticker — must be ignored, so Await never fires.
	h.Handle(delivery(`{"ticker":"GAZP","market":"TQBR","ask_prices":["100"]}`))
	assertNotReady(t, h.Book.Await)
}

func TestOrderBookHandler_FiltersForeignMarket(t *testing.T) {
	h := awaiting.OrderBookHandler{
		Ticker: "SBER", Market: "TQBR",
		Book: awaiting.NewRefreshingValue[awaiting.OrderBook](),
	}
	h.Handle(delivery(`{"ticker":"SBER","market":"SPBX","ask_prices":["100"]}`))
	assertNotReady(t, h.Book.Await)
}

func TestOrderBookHandler_IgnoresGarbage(t *testing.T) {
	h := awaiting.OrderBookHandler{
		Ticker: "SBER", Market: "TQBR",
		Book: awaiting.NewRefreshingValue[awaiting.OrderBook](),
	}
	h.Handle(delivery(`not json at all`))
	assertNotReady(t, h.Book.Await)
}

// ---- TickHandler ----

func TestTickHandler_MatchingMessage(t *testing.T) {
	h := awaiting.TickHandler{
		Ticker: "SBER", Market: "TQBR",
		Tick: awaiting.NewRefreshingValue[awaiting.Tick](),
	}
	h.Handle(delivery(`{"securityId":"SBER","securityExchange":"TQBR","price":"303.10"}`))

	tick := h.Tick.Get()
	if !tick.Price.Equal(decimal.RequireFromString("303.10")) {
		t.Errorf("tick price = %s, want 303.10", tick.Price)
	}
}

func TestTickHandler_FiltersForeignInstrument(t *testing.T) {
	h := awaiting.TickHandler{
		Ticker: "SBER", Market: "TQBR",
		Tick: awaiting.NewRefreshingValue[awaiting.Tick](),
	}
	h.Handle(delivery(`{"securityId":"GAZP","securityExchange":"TQBR","price":"100"}`))
	assertNotReady(t, h.Tick.Await)
}

// ---- StatusHandler ----

func TestStatusHandler_FirstStatusSetsOrderID(t *testing.T) {
	ready := make(chan struct{})
	h := &awaiting.StatusHandler{
		Ctx:               context.Background(),
		OrderName:         "myorder",
		OrderIDReady:      ready,
		StatusLots:        awaiting.NewRefreshingValue[int64](),
		TerminationStatus: make(chan string, 1),
	}
	h.StatusLots.Store(0)

	h.Handle(delivery(`{"strategyName":"myorder","orderId":777,"remainingQty":"10","status":"Submitted"}`))

	select {
	case <-ready:
	default:
		t.Fatal("OrderIDReady should be closed after first matching status")
	}
	if h.OrderID != 777 {
		t.Errorf("OrderID = %d, want 777", h.OrderID)
	}
	if got := h.StatusLots.Get(); got != 10 {
		t.Errorf("StatusLots = %d, want 10", got)
	}
}

func TestStatusHandler_FiltersForeignStrategy(t *testing.T) {
	ready := make(chan struct{})
	h := &awaiting.StatusHandler{
		Ctx:               context.Background(),
		OrderName:         "myorder",
		OrderIDReady:      ready,
		StatusLots:        awaiting.NewRefreshingValue[int64](),
		TerminationStatus: make(chan string, 1),
	}
	h.StatusLots.Store(0)

	// Different strategy name → ignored entirely.
	h.Handle(delivery(`{"strategyName":"someone-else","orderId":1,"remainingQty":"5","status":"Filled"}`))

	select {
	case <-ready:
		t.Fatal("OrderIDReady must not close for a foreign strategy")
	default:
	}
	if len(h.TerminationStatus) != 0 {
		t.Fatal("foreign status must not be sent to TerminationStatus")
	}
}

func TestStatusHandler_TerminalStatusSent(t *testing.T) {
	ready := make(chan struct{})
	term := make(chan string, 1)
	h := &awaiting.StatusHandler{
		Ctx:               context.Background(),
		OrderName:         "myorder",
		OrderIDReady:      ready,
		StatusLots:        awaiting.NewRefreshingValue[int64](),
		TerminationStatus: term,
	}
	h.StatusLots.Store(0)

	h.Handle(delivery(`{"strategyName":"myorder","orderId":5,"remainingQty":"0","status":"Filled"}`))

	select {
	case s := <-term:
		if s != "Filled" {
			t.Errorf("terminal status = %q, want Filled", s)
		}
	default:
		t.Fatal("expected Filled on TerminationStatus")
	}
}

func TestStatusHandler_SubmittedNotTerminal(t *testing.T) {
	h := &awaiting.StatusHandler{
		Ctx:               context.Background(),
		OrderName:         "myorder",
		OrderIDReady:      make(chan struct{}),
		StatusLots:        awaiting.NewRefreshingValue[int64](),
		TerminationStatus: make(chan string, 1),
	}
	h.StatusLots.Store(0)

	h.Handle(delivery(`{"strategyName":"myorder","orderId":5,"remainingQty":"7","status":"Submitted"}`))

	if len(h.TerminationStatus) != 0 {
		t.Fatal("Submitted must not be treated as terminal")
	}
}

// OrderID is latched on the first status; later statuses keep the original ID.
func TestStatusHandler_OrderIDLatched(t *testing.T) {
	h := &awaiting.StatusHandler{
		Ctx:               context.Background(),
		OrderName:         "myorder",
		OrderIDReady:      make(chan struct{}),
		StatusLots:        awaiting.NewRefreshingValue[int64](),
		TerminationStatus: make(chan string, 2),
	}
	h.StatusLots.Store(0)

	h.Handle(delivery(`{"strategyName":"myorder","orderId":100,"remainingQty":"10","status":"Submitted"}`))
	h.Handle(delivery(`{"strategyName":"myorder","orderId":999,"remainingQty":"4","status":"Submitted"}`))

	if h.OrderID != 100 {
		t.Errorf("OrderID = %d, want 100 (must not change after first status)", h.OrderID)
	}
	if got := h.StatusLots.Get(); got != 4 {
		t.Errorf("StatusLots = %d, want 4 (latest remaining)", got)
	}
}

// ---- TradeHandler ----

func TestTradeHandler_AppliesMatchingTrade(t *testing.T) {
	ready := make(chan struct{})
	close(ready) // order id already known
	orderID := int64(42)

	h := &awaiting.TradeHandler{
		Ctx:          context.Background(),
		OrderIDReady: ready,
		OrderID:      &orderID,
		State:        awaiting.NewRefreshingValue[awaiting.OrderState](),
	}
	h.State.Store(awaiting.OrderState{LotsRemaining: 10})

	h.Handle(delivery(`{"orderid":42,"qty":"3","volume":"909.00"}`))

	st := h.State.Get()
	if st.LotsProcessed != 3 || st.LotsRemaining != 7 {
		t.Errorf("state = %+v, want processed 3 / remaining 7", st)
	}
	if !st.TotalCashflow.Equal(decimal.RequireFromString("909.00")) {
		t.Errorf("cashflow = %s, want 909.00", st.TotalCashflow)
	}
}

func TestTradeHandler_FiltersForeignOrderID(t *testing.T) {
	ready := make(chan struct{})
	close(ready)
	orderID := int64(42)

	h := &awaiting.TradeHandler{
		Ctx:          context.Background(),
		OrderIDReady: ready,
		OrderID:      &orderID,
		State:        awaiting.NewRefreshingValue[awaiting.OrderState](),
	}
	h.State.Store(awaiting.OrderState{LotsRemaining: 10})

	h.Handle(delivery(`{"orderid":99,"qty":"3","volume":"900"}`))

	st := h.State.Get()
	if st.LotsProcessed != 0 || st.TotalTrades != 0 {
		t.Errorf("foreign trade was applied: %+v", st)
	}
}

// A sell trade carries negative qty; the handler must take the absolute value
// so that LotsProcessed counts up.
func TestTradeHandler_NegativeQtyAbs(t *testing.T) {
	ready := make(chan struct{})
	close(ready)
	orderID := int64(42)

	h := &awaiting.TradeHandler{
		Ctx:          context.Background(),
		OrderIDReady: ready,
		OrderID:      &orderID,
		State:        awaiting.NewRefreshingValue[awaiting.OrderState](),
	}
	h.State.Store(awaiting.OrderState{LotsRemaining: 10})

	h.Handle(delivery(`{"orderid":42,"qty":"-5","volume":"1500"}`))

	st := h.State.Get()
	if st.LotsProcessed != 5 {
		t.Errorf("LotsProcessed = %d, want 5 (abs of -5)", st.LotsProcessed)
	}
	if st.LotsRemaining != 5 {
		t.Errorf("LotsRemaining = %d, want 5", st.LotsRemaining)
	}
}

// Before the order id is known, a trade must block (not be applied) until the
// context is cancelled.
func TestTradeHandler_WaitsForOrderID(t *testing.T) {
	ready := make(chan struct{}) // never closed
	orderID := int64(42)
	ctx, cancel := context.WithCancel(context.Background())

	h := &awaiting.TradeHandler{
		Ctx:          ctx,
		OrderIDReady: ready,
		OrderID:      &orderID,
		State:        awaiting.NewRefreshingValue[awaiting.OrderState](),
	}
	h.State.Store(awaiting.OrderState{LotsRemaining: 10})

	done := make(chan struct{})
	go func() {
		h.Handle(delivery(`{"orderid":42,"qty":"3","volume":"900"}`))
		close(done)
	}()

	select {
	case <-done:
		t.Fatal("Handle returned before order id was ready")
	case <-time.After(50 * time.Millisecond):
	}

	cancel() // unblock via ctx.Done

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Handle did not return after ctx cancel")
	}

	if st := h.State.Get(); st.LotsProcessed != 0 {
		t.Errorf("trade applied despite cancel: %+v", st)
	}
}

// assertNotReady fails if the RefreshingValue's Await channel has been closed
// (i.e. a value was stored when it should have been filtered out).
func assertNotReady(t *testing.T, await chan struct{}) {
	t.Helper()
	select {
	case <-await:
		t.Fatal("value was stored but the message should have been ignored")
	default:
	}
}
