package test

import (
	"testing"

	"vanilla/internal/ordering/awaiting"

	"github.com/shopspring/decimal"
)

func TestWithTrade_SingleFill(t *testing.T) {
	s := awaiting.OrderState{LotsRemaining: 10}
	s = s.WithTrade(4, decimal.RequireFromString("1212.00"))

	if s.LotsProcessed != 4 {
		t.Errorf("LotsProcessed = %d, want 4", s.LotsProcessed)
	}
	if s.LotsRemaining != 6 {
		t.Errorf("LotsRemaining = %d, want 6", s.LotsRemaining)
	}
	if s.TotalTrades != 1 {
		t.Errorf("TotalTrades = %d, want 1", s.TotalTrades)
	}
	if !s.TotalCashflow.Equal(decimal.RequireFromString("1212.00")) {
		t.Errorf("TotalCashflow = %s, want 1212.00", s.TotalCashflow)
	}
}

func TestWithTrade_Accumulates(t *testing.T) {
	s := awaiting.OrderState{LotsRemaining: 10}
	s = s.WithTrade(4, decimal.RequireFromString("1200"))
	s = s.WithTrade(6, decimal.RequireFromString("1830"))

	if s.LotsProcessed != 10 {
		t.Errorf("LotsProcessed = %d, want 10", s.LotsProcessed)
	}
	if s.LotsRemaining != 0 {
		t.Errorf("LotsRemaining = %d, want 0", s.LotsRemaining)
	}
	if s.TotalTrades != 2 {
		t.Errorf("TotalTrades = %d, want 2", s.TotalTrades)
	}
	if !s.TotalCashflow.Equal(decimal.RequireFromString("3030")) {
		t.Errorf("TotalCashflow = %s, want 3030", s.TotalCashflow)
	}
}

// WithTrade must not mutate the receiver — it returns a fresh copy.
func TestWithTrade_DoesNotMutateReceiver(t *testing.T) {
	orig := awaiting.OrderState{LotsRemaining: 10}
	_ = orig.WithTrade(3, decimal.RequireFromString("900"))

	if orig.LotsProcessed != 0 || orig.LotsRemaining != 10 || orig.TotalTrades != 0 {
		t.Errorf("receiver was mutated: %+v", orig)
	}
	if !orig.TotalCashflow.IsZero() {
		t.Errorf("receiver cashflow mutated: %s", orig.TotalCashflow)
	}
}

// Overfill (more lots than remaining) drives LotsRemaining negative — Monitor
// relies on the <= 0 check, so a negative result must be representable.
func TestWithTrade_Overfill(t *testing.T) {
	s := awaiting.OrderState{LotsRemaining: 5}
	s = s.WithTrade(8, decimal.RequireFromString("2400"))

	if s.LotsRemaining != -3 {
		t.Errorf("LotsRemaining = %d, want -3", s.LotsRemaining)
	}
	if s.LotsProcessed != 8 {
		t.Errorf("LotsProcessed = %d, want 8", s.LotsProcessed)
	}
}
