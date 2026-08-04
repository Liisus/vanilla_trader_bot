package app

import (
	"fmt"

	"github.com/shopspring/decimal"
	"github.com/urfave/cli/v2"

	"vanilla/internal/app/query"
	"vanilla/internal/ordering"
)

func MakeApp() *cli.App {
	return &cli.App{
		Name:            "vanilla",
		Usage:           "Simple trade bot",
		HideHelpCommand: true,
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "ticker", Required: true, Aliases: []string{"t"}},
			&cli.StringFlag{Name: "market", Required: true, Aliases: []string{"m"}},
			&cli.Int64Flag{Name: "qty", Aliases: []string{"q"}},
			&cli.StringFlag{Name: "volume", Aliases: []string{"v"}},
			&cli.StringFlag{Name: "price", Aliases: []string{"p"}},
			&cli.StringFlag{Name: "slippage", Aliases: []string{"s"}},
			&cli.BoolFlag{Name: "best_quote", Aliases: []string{"b"}},
			&cli.StringFlag{Name: "config", Aliases: []string{"c"}, Value: "./config/data/", Usage: "directory with config.json and securities/"},
		},
		Action: run,
		OnUsageError: func(_ *cli.Context, err error, _ bool) error {
			return cli.Exit(err, 2)
		},
	}
}

func run(c *cli.Context) error {
	query, err := query.ParseQuery(c)
	if err != nil {
		return err
	}

	report, err := ordering.PlaceOrder(c.Context, &query)
	if err != nil {
		return err
	}

	if report.ProcessedLots != 0 {
		fmt.Printf("Ордер %d (%s@%s, %s) [%s]: исполнено %d лотов на %s RUB, средняя цена %s. Количество сделок: %d\n",
			report.OrderID, report.Ticker, report.Market, report.Side, report.Status, report.ProcessedLots, report.Volume.String(),
			report.Volume.Div(decimal.NewFromInt(report.ProcessedLots).Mul(report.LotSize)).String(), report.ProcessedTrades)
	} else {
		fmt.Printf("Ордер %d (%s@%s, %s) [%s]: исполнено 0 лотов на 0.00 RUB\n",
			report.OrderID, report.Ticker, report.Market, report.Side, report.Status)
	}
	return nil
}
