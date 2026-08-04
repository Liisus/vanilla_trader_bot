package test

import (
	"flag"
	"testing"

	"vanilla/internal/app"
	"vanilla/internal/app/query"

	"github.com/shopspring/decimal"
	"github.com/urfave/cli/v2"
)

// parseArgs builds a cli.Context from the real app flag definitions and runs
// ParseQuery against it. Always use full flag names (not aliases) so that the
// flag set records them as "set".
func parseArgs(t *testing.T, args []string) (query.Query, error) {
	t.Helper()
	a := app.MakeApp()
	fs := flag.NewFlagSet("vanilla", flag.ContinueOnError)
	for _, f := range a.Flags {
		if err := f.Apply(fs); err != nil {
			t.Fatalf("apply flag: %v", err)
		}
	}
	if err := fs.Parse(args); err != nil {
		t.Fatalf("parse args: %v", err)
	}
	ctx := cli.NewContext(a, fs, nil)
	return query.ParseQuery(ctx)
}

func TestParseQuery_QtyAndPrice(t *testing.T) {
	dir := writeConfigDir(t, configFixture{})
	q, err := parseArgs(t, []string{
		"-ticker", "SBER", "-market", "TQBR", "-qty", "10", "-price", "303.15", "-config", dir,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !q.IsSet("qty") || q.IsSet("volume") {
		t.Errorf("expected qty set, volume unset")
	}
	if !q.IsSet("price") {
		t.Error("expected price set")
	}
	if q.Qty != 10 {
		t.Errorf("Qty = %d, want 10", q.Qty)
	}
	if !q.Price.Equal(decimal.RequireFromString("303.15")) {
		t.Errorf("Price = %s, want 303.15", q.Price)
	}
	// Security params filled from config.
	if !q.LotSize.Equal(decimal.NewFromInt(1)) {
		t.Errorf("LotSize = %s, want 1", q.LotSize)
	}
	if !q.MinStep.Equal(decimal.RequireFromString("0.01")) {
		t.Errorf("MinStep = %s, want 0.01", q.MinStep)
	}
}

func TestParseQuery_VolumeAndSlippage(t *testing.T) {
	dir := writeConfigDir(t, configFixture{})
	q, err := parseArgs(t, []string{
		"-ticker", "CNYRUB_TOM", "-market", "CETS", "-volume", "-50000", "-slippage", "0.1", "-config", dir,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !q.IsSet("volume") || q.IsSet("qty") {
		t.Errorf("expected volume set, qty unset")
	}
	if !q.IsSet("slippage") {
		t.Error("expected slippage set")
	}
	if !q.Volume.Equal(decimal.RequireFromString("-50000")) {
		t.Errorf("Volume = %s, want -50000", q.Volume)
	}
	if !q.LotSize.Equal(decimal.NewFromInt(1000)) {
		t.Errorf("LotSize = %s, want 1000", q.LotSize)
	}
}

func TestParseQuery_BestQuote(t *testing.T) {
	dir := writeConfigDir(t, configFixture{})
	q, err := parseArgs(t, []string{
		"-ticker", "SBER", "-market", "TQBR", "-qty", "1", "-best_quote", "-config", dir,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !q.IsSet("best_quote") || !q.BestQuote {
		t.Error("expected best_quote set")
	}
}

// qty takes priority over volume when both are supplied.
func TestParseQuery_QtyBeatsVolume(t *testing.T) {
	dir := writeConfigDir(t, configFixture{})
	q, err := parseArgs(t, []string{
		"-ticker", "SBER", "-market", "TQBR", "-qty", "5", "-volume", "1000", "-price", "300", "-config", dir,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !q.IsSet("qty") || q.IsSet("volume") {
		t.Error("qty must win over volume")
	}
}

// price takes priority over slippage and best_quote.
func TestParseQuery_PriceBeatsSlippage(t *testing.T) {
	dir := writeConfigDir(t, configFixture{})
	q, err := parseArgs(t, []string{
		"-ticker", "SBER", "-market", "TQBR", "-qty", "5", "-price", "300", "-slippage", "0.5", "-config", dir,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !q.IsSet("price") || q.IsSet("slippage") {
		t.Error("price must win over slippage")
	}
}

func TestParseQuery_MissingVolumeFlags(t *testing.T) {
	dir := writeConfigDir(t, configFixture{})
	_, err := parseArgs(t, []string{
		"-ticker", "SBER", "-market", "TQBR", "-price", "300", "-config", dir,
	})
	if err == nil {
		t.Fatal("expected error: neither qty nor volume given")
	}
}

func TestParseQuery_MissingPriceFlags(t *testing.T) {
	dir := writeConfigDir(t, configFixture{})
	_, err := parseArgs(t, []string{
		"-ticker", "SBER", "-market", "TQBR", "-qty", "10", "-config", dir,
	})
	if err == nil {
		t.Fatal("expected error: no price/slippage/best_quote given")
	}
}

func TestParseQuery_UnknownSecurity(t *testing.T) {
	dir := writeConfigDir(t, configFixture{})
	_, err := parseArgs(t, []string{
		"-ticker", "GAZP", "-market", "TQBR", "-qty", "10", "-price", "100", "-config", dir,
	})
	if err == nil {
		t.Fatal("expected error for instrument not in config")
	}
}

func TestParseQuery_InvalidVolumeDecimal(t *testing.T) {
	dir := writeConfigDir(t, configFixture{})
	_, err := parseArgs(t, []string{
		"-ticker", "SBER", "-market", "TQBR", "-volume", "not-a-number", "-price", "100", "-config", dir,
	})
	if err == nil {
		t.Fatal("expected error for invalid --volume")
	}
}

func TestParseQuery_InvalidPriceDecimal(t *testing.T) {
	dir := writeConfigDir(t, configFixture{})
	_, err := parseArgs(t, []string{
		"-ticker", "SBER", "-market", "TQBR", "-qty", "10", "-price", "abc", "-config", dir,
	})
	if err == nil {
		t.Fatal("expected error for invalid --price")
	}
}

func TestParseQuery_BadConfigDir(t *testing.T) {
	_, err := parseArgs(t, []string{
		"-ticker", "SBER", "-market", "TQBR", "-qty", "10", "-price", "100", "-config", "/no/such/dir",
	})
	if err == nil {
		t.Fatal("expected error for missing config directory")
	}
}

// Negative qty is accepted at parse time (sign denotes sell side); the value is
// preserved verbatim.
func TestParseQuery_NegativeQty(t *testing.T) {
	dir := writeConfigDir(t, configFixture{})
	q, err := parseArgs(t, []string{
		"-ticker", "SBER", "-market", "TQBR", "-qty", "-7", "-price", "300", "-config", dir,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if q.Qty != -7 {
		t.Errorf("Qty = %d, want -7", q.Qty)
	}
}
