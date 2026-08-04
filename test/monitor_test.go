package test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"vanilla/internal/ordering/awaiting"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/shopspring/decimal"
)

type monitorResult struct {
	state  awaiting.OrderState
	status string
}

// monitorHarness wires a Monitor to two in-memory channels that stand in for
// the RabbitMQ statuses/trades streams, and runs Wait in the background.
type monitorHarness struct {
	statusCh chan amqp.Delivery
	tradeCh  chan amqp.Delivery
	result   chan monitorResult
	monitor  *awaiting.Monitor
	cancel   context.CancelFunc
}

func newMonitorHarness(t *testing.T, orderName string, lots int64) *monitorHarness {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())

	statusCh := make(chan amqp.Delivery)
	tradeCh := make(chan amqp.Delivery)
	b := awaiting.Broker{Statuses: statusCh, Trades: tradeCh}

	m := awaiting.StartMonitor(ctx, &b, orderName, lots)

	res := make(chan monitorResult, 1)
	go func() {
		state, status := m.Wait()
		res <- monitorResult{state: state, status: status}
	}()

	h := &monitorHarness{statusCh: statusCh, tradeCh: tradeCh, result: res, monitor: m, cancel: cancel}
	t.Cleanup(cancel)
	return h
}

func (h *monitorHarness) status(strategy string, orderID, remaining int64, status string) {
	h.statusCh <- delivery(fmt.Sprintf(
		`{"strategyName":%q,"orderId":%d,"remainingQty":"%d","status":%q}`,
		strategy, orderID, remaining, status))
}

func (h *monitorHarness) trade(orderID, qty int64, volume string) {
	h.tradeCh <- delivery(fmt.Sprintf(
		`{"orderid":%d,"qty":"%d","volume":%q}`, orderID, qty, volume))
}

func (h *monitorHarness) await(t *testing.T) monitorResult {
	t.Helper()
	select {
	case r := <-h.result:
		return r
	case <-time.After(2 * time.Second):
		t.Fatal("Monitor.Wait did not return (possible deadlock)")
		return monitorResult{}
	}
}

// Order fully filled by trades, before any terminal status arrives.
func TestMonitor_FulfilledByTrades(t *testing.T) {
	h := newMonitorHarness(t, "ord", 10)

	h.status("ord", 42, 10, "Submitted") // assigns order id
	h.trade(42, 10, "3030.00")           // fills everything

	r := h.await(t)
	if r.status != "Fulfilled" {
		t.Errorf("status = %q, want Fulfilled", r.status)
	}
	if r.state.LotsProcessed != 10 || r.state.LotsRemaining != 0 {
		t.Errorf("state = %+v, want processed 10 / remaining 0", r.state)
	}
	if !r.state.TotalCashflow.Equal(decimal.RequireFromString("3030.00")) {
		t.Errorf("cashflow = %s, want 3030.00", r.state.TotalCashflow)
	}
}

// Cancelled with nothing filled.
func TestMonitor_CancelledNoFill(t *testing.T) {
	h := newMonitorHarness(t, "ord", 10)

	h.status("ord", 42, 10, "Submitted")
	h.status("ord", 42, 10, "Cancelled")

	r := h.await(t)
	if r.status != "Cancelled" {
		t.Errorf("status = %q, want Cancelled", r.status)
	}
	if r.state.LotsProcessed != 0 {
		t.Errorf("LotsProcessed = %d, want 0", r.state.LotsProcessed)
	}
}

// Partial fill, then Cancelled — and the status reports fewer remaining lots
// than the trades have accounted for, so reconcile must wait for the lagging
// trade before reporting the final state.
func TestMonitor_CancelledReconcileLaggingTrade(t *testing.T) {
	h := newMonitorHarness(t, "ord", 10)

	h.status("ord", 42, 10, "Submitted")
	h.trade(42, 4, "1200.00") // trades know about 4
	h.status("ord", 42, 4, "Cancelled")
	h.trade(42, 2, "600.00") // the lagging 6th-lot batch arrives late

	r := h.await(t)
	if r.status != "Cancelled" {
		t.Errorf("status = %q, want Cancelled", r.status)
	}
	if r.state.LotsProcessed != 6 {
		t.Errorf("LotsProcessed = %d, want 6 (reconciled)", r.state.LotsProcessed)
	}
	if r.state.TotalTrades != 2 {
		t.Errorf("TotalTrades = %d, want 2", r.state.TotalTrades)
	}
	if !r.state.TotalCashflow.Equal(decimal.RequireFromString("1800.00")) {
		t.Errorf("cashflow = %s, want 1800.00", r.state.TotalCashflow)
	}
}

// Sell order: trades carry negative qty, the monitor counts absolute lots.
func TestMonitor_SellFilledByTrades(t *testing.T) {
	h := newMonitorHarness(t, "ord", 5)

	h.status("ord", 99, 5, "Submitted")
	h.trade(99, -5, "1515.00")

	r := h.await(t)
	if r.status != "Fulfilled" {
		t.Errorf("status = %q, want Fulfilled", r.status)
	}
	if r.state.LotsProcessed != 5 {
		t.Errorf("LotsProcessed = %d, want 5", r.state.LotsProcessed)
	}
}

// OrderID() exposes the id latched from the first status.
func TestMonitor_OrderIDReported(t *testing.T) {
	h := newMonitorHarness(t, "ord", 1)

	h.status("ord", 555, 1, "Submitted")
	h.trade(555, 1, "303.15")

	_ = h.await(t)
	if id := h.monitor.OrderID(); id != 555 {
		t.Errorf("OrderID() = %d, want 555", id)
	}
}
