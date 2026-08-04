package awaiting

import "context"

type Monitor struct {
	status *StatusHandler
	trades *TradeHandler
	lots   int64
}

func StartMonitor(ctx context.Context, b *Broker, orderName string, lots int64) *Monitor {
	orderIDReady := make(chan struct{})

	status := &StatusHandler{
		Ctx:               ctx,
		OrderName:         orderName,
		OrderIDReady:      orderIDReady,
		StatusLots:        NewRefreshingValue[int64](),
		TerminationStatus: make(chan string),
	}
	status.StatusLots.Store(0)

	trades := &TradeHandler{
		Ctx:          ctx,
		OrderIDReady: orderIDReady,
		OrderID:      &status.OrderID,
		State:        NewRefreshingValue[OrderState](),
	}
	trades.State.Store(OrderState{LotsRemaining: lots})

	go Refresher(ctx, b.Statuses, status.Handle)
	go Refresher(ctx, b.Trades, trades.Handle)

	return &Monitor{status: status, trades: trades, lots: lots}
}

func (m *Monitor) OrderID() int64 { return m.status.OrderID }

func (m *Monitor) Wait() (OrderState, string) {
	finalStatus := ""
	state := OrderState{LotsRemaining: m.lots}

	for {
		select {
		case finalStatus = <-m.status.TerminationStatus:
			return m.reconcile(state, finalStatus)
		case <-m.trades.State.Await:
			state = m.trades.State.Get()
			if state.LotsRemaining <= 0 {
				return m.reconcile(state, "Fulfilled")
			}
		}
	}
}

func (m *Monitor) reconcile(state OrderState, finalStatus string) (OrderState, string) {
	reportedRemaining := m.status.StatusLots.AwaitNew()
	for state.LotsRemaining > reportedRemaining {
		state = m.trades.State.AwaitNew()
	}
	return state, finalStatus
}
