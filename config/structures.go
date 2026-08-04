package config

import (
	"time"

	"github.com/shopspring/decimal"
)

type rawURL struct {
	URL          string `json:"url"`
	AwaitTimeout string `json:"awaitTimeout"`
}

type Publisher struct {
	Exchange   string `json:"exchange"`
	RoutingKey string `json:"routingKey"`
}

type Consumer struct {
	QueueName  string `json:"queueName"`
	Exchange   string `json:"exchange"`
	RoutingKey string `json:"routingKey"`
	MaxLength  int64  `json:"maxLength"`
}

type rawQueues struct {
	Publisher Publisher           `json:"publisher"`
	Consumers map[string]Consumer `json:"consumers"`
}

type rawSecurityData struct {
	SecID   string          `json:"SECID"`
	BoardID string          `json:"BOARDID"`
	LotSize decimal.Decimal `json:"LOTSIZE"`
	MinStep decimal.Decimal `json:"MINSTEP"`
}

type Security struct {
	Market, Ticker string
}

type SecurityData struct {
	LotSize decimal.Decimal
	MinStep decimal.Decimal
}

type Config struct {
	URL          string
	AwaitTimeout time.Duration
	Securities   map[Security]SecurityData

	Publisher Publisher
	Consumers map[string]Consumer
}
