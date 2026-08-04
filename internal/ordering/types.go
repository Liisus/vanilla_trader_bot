package ordering

import (
	"github.com/shopspring/decimal"
)

// broker groups everything the bot talks to over RabbitMQ:
// one publish function and four consume streams.

type Report struct {
	OrderID int64
	Ticker  string
	Market  string

	Side string

	ProcessedLots   int64
	ProcessedTrades int64
	Volume          decimal.Decimal

	LotSize decimal.Decimal

	Status string
}

type order struct {
	name   string
	price  decimal.Decimal
	qty    int64
	ticker string
	market string
}
