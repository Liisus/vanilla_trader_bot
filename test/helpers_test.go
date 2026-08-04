package test

import (
	"os"
	"path/filepath"
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"
)

// validConfigJSON is a minimal, well-formed config.json body.
const validConfigJSON = `{
  "url": "amqp://user:pass@localhost:5672/",
  "awaitTimeout": "5s"
}`

// validQueuesJSON is a minimal, well-formed queues.json body.
const validQueuesJSON = `{
  "publisher": {"exchange": "sandbox.orders", "routingKey": "locko.place"},
  "consumers": {
    "statuses":   {"queueName": "q.statuses",   "exchange": "sandbox.order.status",     "routingKey": "#", "maxLength": 1000},
    "trades":     {"queueName": "q.trades",     "exchange": "sandbox.trades",           "routingKey": "#", "maxLength": 1000},
    "orderbooks": {"queueName": "q.orderbooks", "exchange": "marketdata.orderbooks.alor","routingKey": "#", "maxLength": 1000},
    "ticks":      {"queueName": "q.ticks",      "exchange": "marketdata.ticks.alor",     "routingKey": "#", "maxLength": 1000}
  }
}`

// validSecuritiesJSON describes SBER (lot 1, step 0.01) and CNYRUB_TOM (lot 1000, step 0.0005).
const validSecuritiesJSON = `[
  {"SECID": "SBER",       "BOARDID": "TQBR", "LOTSIZE": 1,    "MINSTEP": 0.01},
  {"SECID": "CNYRUB_TOM", "BOARDID": "CETS", "LOTSIZE": 1000, "MINSTEP": 0.0005}
]`

// configFixture lets a test override individual pieces of a config directory.
// Empty string fields fall back to the valid defaults; the *Omit flags drop a
// file entirely so that "missing file" error paths can be exercised.
type configFixture struct {
	configJSON     string
	queuesJSON     string
	securitiesJSON string

	omitConfig     bool
	omitQueues     bool
	omitSecurities bool
}

// writeConfigDir materialises a configFixture into a fresh temp directory and
// returns its path. t.TempDir handles cleanup.
func writeConfigDir(t *testing.T, f configFixture) string {
	t.Helper()
	dir := t.TempDir()

	if !f.omitConfig {
		body := f.configJSON
		if body == "" {
			body = validConfigJSON
		}
		writeFile(t, filepath.Join(dir, "config.json"), body)
	}

	if !f.omitQueues {
		body := f.queuesJSON
		if body == "" {
			body = validQueuesJSON
		}
		writeFile(t, filepath.Join(dir, "queues.json"), body)
	}

	if !f.omitSecurities {
		secDir := filepath.Join(dir, "securities")
		if err := os.MkdirAll(secDir, 0o755); err != nil {
			t.Fatalf("mkdir securities: %v", err)
		}
		body := f.securitiesJSON
		if body == "" {
			body = validSecuritiesJSON
		}
		writeFile(t, filepath.Join(secDir, "all.json"), body)
	}

	return dir
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// delivery wraps a raw JSON payload as an AMQP delivery, the way the broker
// hands messages to the handlers.
func delivery(body string) amqp.Delivery {
	return amqp.Delivery{Body: []byte(body)}
}
