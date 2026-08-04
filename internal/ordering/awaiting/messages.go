// Wire formats of RabbitMQ messages (see task/vanilla.md, sections 5-8).
package awaiting

import "github.com/shopspring/decimal"

type Tick struct {
	TradeID          int64           `json:"tradeid"`
	Time             string          `json:"time"`
	DirSide          int64           `json:"dirSide"`
	SecurityID       string          `json:"securityId"`
	SecurityExchange string          `json:"securityExchange"`
	Price            decimal.Decimal `json:"price"`
	Qty              decimal.Decimal `json:"qty"`
	Volume           decimal.Decimal `json:"volume"`
	AccruedInt       decimal.Decimal `json:"accruedint"`
}

type OrderBook struct {
	Time       string                `json:"time"`
	Ticker     string                `json:"ticker"`
	Market     string                `json:"market"`
	BidPrices  []decimal.Decimal     `json:"bid_prices"`
	BidVolumes []decimal.Decimal     `json:"bid_volumes"`
	BidYtms    []decimal.NullDecimal `json:"bid_ytms"`
	AskPrices  []decimal.Decimal     `json:"ask_prices"`
	AskVolumes []decimal.Decimal     `json:"ask_volumes"`
	AskYtms    []decimal.NullDecimal `json:"ask_ytms"`
}

type OrderStatus struct {
	StrategyName string              `json:"strategyName"`
	OrderID      int64               `json:"orderId"`
	SecurityName string              `json:"securityName"`
	FilledQty    decimal.Decimal     `json:"filledQty"`
	RemainingQty decimal.Decimal     `json:"remainingQty"`
	VisibleQty   decimal.NullDecimal `json:"visibleQty"`
	TradeSide    string              `json:"tradeSide"`
	Price        decimal.Decimal     `json:"price"`
	Status       string              `json:"status"`
	Account      string              `json:"account"`
}

type Trade struct {
	DateTime     string          `json:"datetime"`
	TradeID      int64           `json:"tradeid"`
	OrderID      int64           `json:"orderid"`
	TradeSide    string          `json:"tradeside"`
	SecurityName string          `json:"securityname"`
	Price        decimal.Decimal `json:"price"`
	Qty          decimal.Decimal `json:"qty"`
	Volume       decimal.Decimal `json:"volume"`
	AccruedInt   decimal.Decimal `json:"accruedint"`
	BrokerRef    string          `json:"brokerref"`
	UserID       string          `json:"userid"`
}

type OrderRequest struct {
	OrderStrategyID string          `json:"OrderStrategyId"`
	OrderSecurityID string          `json:"OrderSecurityId"`
	OrderType       string          `json:"OrderType"`
	OrderPrice      decimal.Decimal `json:"OrderPrice"`
	OrderQty        int64           `json:"OrderQty"`
	OrderPortfolio  string          `json:"OrderPortfolio"`
	OrderClientCode string          `json:"OrderClientCode"`
}
