package query

import (
	"fmt"
	"vanilla/config"

	"github.com/shopspring/decimal"
	"github.com/urfave/cli/v2"
)

type Query struct {
	config.Config

	Ticker string
	Market string

	LotSize decimal.Decimal
	MinStep decimal.Decimal

	Qty    int64
	Volume decimal.Decimal

	Price     decimal.Decimal
	Slippage  decimal.Decimal
	BestQuote bool

	set map[string]struct{}
}

func (q *Query) MarkSet(name string) {
	q.set[name] = struct{}{}
}

func (q Query) IsSet(name string) bool {
	_, ok := q.set[name]
	return ok
}

func ParseQuery(c *cli.Context) (Query, error) {
	query := Query{
		Ticker: c.String("ticker"),
		Market: c.String("market"),
		set:    make(map[string]struct{}),
	}

	cfg, err := config.LoadConfig(c.String("config"))
	if err != nil {
		return Query{}, err
	}

	query.Config = cfg

	switch {
	case c.IsSet("qty"):
		query.Qty = c.Int64("qty")
		query.MarkSet("qty")
	case c.IsSet("volume"):
		volume, err := decimalFlag(c, "volume")
		if err != nil {
			return Query{}, err
		}
		query.Volume = volume
		query.MarkSet("volume")
	default:
		return Query{}, cli.Exit("Specify --qty or --volume", 2)
	}

	switch {
	case c.IsSet("price"):
		price, err := decimalFlag(c, "price")
		if err != nil {
			return Query{}, err
		}
		query.Price = price
		query.MarkSet("price")
	case c.IsSet("slippage"):
		slippage, err := decimalFlag(c, "slippage")
		if err != nil {
			return Query{}, err
		}
		query.Slippage = slippage
		query.MarkSet("slippage")
	case c.IsSet("best_quote"):
		query.BestQuote = true
		query.MarkSet("best_quote")
	default:
		return Query{}, cli.Exit("Specify --price, --slippage or --best_quote", 2)
	}

	params, ok := cfg.Securities[config.Security{Market: query.Market, Ticker: query.Ticker}]
	if !ok {
		return Query{}, fmt.Errorf("You can't trade %s at %s", query.Ticker, query.Market)
	}
	query.LotSize = params.LotSize
	query.MinStep = params.MinStep

	return query, nil
}

func decimalFlag(c *cli.Context, name string) (decimal.Decimal, error) {
	value, err := decimal.NewFromString(c.String(name))
	if err != nil {
		return decimal.Decimal{}, fmt.Errorf("invalid --%s: %w", name, err)
	}
	return value, nil
}
