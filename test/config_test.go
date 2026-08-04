package test

import (
	"testing"
	"time"

	"vanilla/config"

	"github.com/shopspring/decimal"
)

func TestLoadConfig_Valid(t *testing.T) {
	dir := writeConfigDir(t, configFixture{})

	cfg, err := config.LoadConfig(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.URL != "amqp://user:pass@localhost:5672/" {
		t.Errorf("URL = %q", cfg.URL)
	}
	if cfg.AwaitTimeout != 5*time.Second {
		t.Errorf("AwaitTimeout = %v, want 5s", cfg.AwaitTimeout)
	}

	// Both securities present and keyed by {Market, Ticker}.
	sber, ok := cfg.Securities[config.Security{Market: "TQBR", Ticker: "SBER"}]
	if !ok {
		t.Fatal("SBER@TQBR missing from securities")
	}
	if !sber.LotSize.Equal(decimal.NewFromInt(1)) {
		t.Errorf("SBER LotSize = %s, want 1", sber.LotSize)
	}
	if !sber.MinStep.Equal(decimal.RequireFromString("0.01")) {
		t.Errorf("SBER MinStep = %s, want 0.01", sber.MinStep)
	}

	cny, ok := cfg.Securities[config.Security{Market: "CETS", Ticker: "CNYRUB_TOM"}]
	if !ok {
		t.Fatal("CNYRUB_TOM@CETS missing from securities")
	}
	if !cny.LotSize.Equal(decimal.NewFromInt(1000)) {
		t.Errorf("CNY LotSize = %s, want 1000", cny.LotSize)
	}

	// Queue topology parsed.
	if cfg.Publisher.Exchange != "sandbox.orders" || cfg.Publisher.RoutingKey != "locko.place" {
		t.Errorf("publisher = %+v", cfg.Publisher)
	}
	for _, name := range []string{"statuses", "trades", "orderbooks", "ticks"} {
		if _, ok := cfg.Consumers[name]; !ok {
			t.Errorf("consumer %q missing", name)
		}
	}
	if cfg.Consumers["statuses"].MaxLength != 1000 {
		t.Errorf("statuses MaxLength = %d, want 1000", cfg.Consumers["statuses"].MaxLength)
	}
}

func TestLoadConfig_MissingConfigFile(t *testing.T) {
	dir := writeConfigDir(t, configFixture{omitConfig: true})
	if _, err := config.LoadConfig(dir); err == nil {
		t.Fatal("expected error for missing config.json")
	}
}

func TestLoadConfig_NonexistentDir(t *testing.T) {
	if _, err := config.LoadConfig("/no/such/path/at/all"); err == nil {
		t.Fatal("expected error for nonexistent directory")
	}
}

func TestLoadConfig_MalformedConfigJSON(t *testing.T) {
	dir := writeConfigDir(t, configFixture{configJSON: `{ this is not json`})
	if _, err := config.LoadConfig(dir); err == nil {
		t.Fatal("expected error for malformed config.json")
	}
}

func TestLoadConfig_BadAwaitTimeout(t *testing.T) {
	dir := writeConfigDir(t, configFixture{
		configJSON: `{"url":"amqp://x","awaitTimeout":"not-a-duration"}`,
	})
	if _, err := config.LoadConfig(dir); err == nil {
		t.Fatal("expected error for unparsable awaitTimeout")
	}
}

func TestLoadConfig_EmptyAwaitTimeout(t *testing.T) {
	// time.ParseDuration("") is an error, so an empty/omitted field must fail.
	dir := writeConfigDir(t, configFixture{configJSON: `{"url":"amqp://x"}`})
	if _, err := config.LoadConfig(dir); err == nil {
		t.Fatal("expected error for empty awaitTimeout")
	}
}

func TestLoadConfig_MissingQueues(t *testing.T) {
	dir := writeConfigDir(t, configFixture{omitQueues: true})
	if _, err := config.LoadConfig(dir); err == nil {
		t.Fatal("expected error for missing queues.json")
	}
}

func TestLoadConfig_MalformedQueues(t *testing.T) {
	dir := writeConfigDir(t, configFixture{queuesJSON: `{"publisher": 123}`})
	if _, err := config.LoadConfig(dir); err == nil {
		t.Fatal("expected error for malformed queues.json")
	}
}

func TestLoadConfig_MalformedSecurities(t *testing.T) {
	dir := writeConfigDir(t, configFixture{securitiesJSON: `{not an array}`})
	if _, err := config.LoadConfig(dir); err == nil {
		t.Fatal("expected error for malformed securities json")
	}
}

func TestLoadConfig_NonPositiveLotSize(t *testing.T) {
	dir := writeConfigDir(t, configFixture{
		securitiesJSON: `[{"SECID":"X","BOARDID":"Y","LOTSIZE":0,"MINSTEP":0.01}]`,
	})
	if _, err := config.LoadConfig(dir); err == nil {
		t.Fatal("expected error for non-positive lot size")
	}
}

func TestLoadConfig_NegativeLotSize(t *testing.T) {
	dir := writeConfigDir(t, configFixture{
		securitiesJSON: `[{"SECID":"X","BOARDID":"Y","LOTSIZE":-5,"MINSTEP":0.01}]`,
	})
	if _, err := config.LoadConfig(dir); err == nil {
		t.Fatal("expected error for negative lot size")
	}
}

func TestLoadConfig_NonPositiveMinStep(t *testing.T) {
	dir := writeConfigDir(t, configFixture{
		securitiesJSON: `[{"SECID":"X","BOARDID":"Y","LOTSIZE":1,"MINSTEP":0}]`,
	})
	if _, err := config.LoadConfig(dir); err == nil {
		t.Fatal("expected error for non-positive min step")
	}
}

func TestLoadConfig_NoSecuritiesDir(t *testing.T) {
	// Glob over a missing securities dir yields no files (not an error), so the
	// config loads with an empty securities map.
	dir := writeConfigDir(t, configFixture{omitSecurities: true})
	cfg, err := config.LoadConfig(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Securities) != 0 {
		t.Errorf("expected 0 securities, got %d", len(cfg.Securities))
	}
}

func TestLoadConfig_EmptySecuritiesArray(t *testing.T) {
	dir := writeConfigDir(t, configFixture{securitiesJSON: `[]`})
	cfg, err := config.LoadConfig(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Securities) != 0 {
		t.Errorf("expected 0 securities, got %d", len(cfg.Securities))
	}
}
