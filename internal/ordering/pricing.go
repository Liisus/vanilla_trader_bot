package ordering

import (
	"context"
	"errors"
	"fmt"
	"vanilla/internal/app/query"
	"vanilla/internal/ordering/awaiting"

	"github.com/shopspring/decimal"
)

func computePriceQty(ctx context.Context, query query.Query, obh awaiting.OrderBookHandler, th awaiting.TickHandler) (decimal.Decimal, int64, error) {
	side, err := orderSide(query)
	if err != nil {
		return decimal.Decimal{}, 0, err
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, query.AwaitTimeout)
	defer cancel()

	var price decimal.Decimal

	switch {
	case query.IsSet("price"):
		price = query.Price

	case query.IsSet("slippage"):
		tick, err := th.Tick.AwaitNewCtx(timeoutCtx)
		if err != nil {
			return decimal.Decimal{}, 0, fmt.Errorf("waiting for last price: %w", err)
		}
		// price = last * (1 + side*slippage/100)
		slip := decimal.NewFromInt(side).Mul(query.Slippage.Div(decimal.NewFromInt(100)))
		price = tick.Price.Mul(decimal.NewFromInt(1).Add(slip)).Div(query.MinStep).Round(0).Mul(query.MinStep)

	default:
		book, err := obh.Book.AwaitNewCtx(timeoutCtx)
		if err != nil {
			return decimal.Decimal{}, 0, fmt.Errorf("waiting for order book: %w", err)
		}

		if side > 0 {
			if len(book.AskPrices) == 0 {
				return decimal.Decimal{}, 0, errors.New("noone wants to sell")
			}
			price = book.AskPrices[0]
		} else {
			if len(book.BidPrices) == 0 {
				return decimal.Decimal{}, 0, errors.New("noone wants to buy")
			}
			price = book.BidPrices[0]
		}
	}

	var qty int64
	if query.IsSet("qty") {
		qty = query.Qty
	} else {
		qty = query.Volume.Div(price.Mul(query.LotSize)).IntPart()
		if qty == 0 {
			return decimal.Decimal{}, 0, errors.New("volume too small for one lot")
		}
	}

	return price, qty, nil
}

// +1 buy, -1 sell.
func orderSide(query query.Query) (int64, error) {
	if query.IsSet("qty") {
		switch {
		case query.Qty > 0:
			return 1, nil
		case query.Qty < 0:
			return -1, nil
		}
		return 0, errors.New("qty must not be 0")
	}

	switch {
	case query.Volume.IsPositive():
		return 1, nil
	case query.Volume.IsNegative():
		return -1, nil
	}
	return 0, errors.New("volume must not be 0")
}
