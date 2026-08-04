package main

import (
	"fmt"
	"os"
	"vanilla/internal/app"
)

func main() {
	app := app.MakeApp()

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
