package main

import (
	"os"

	"github.com/MoreCodeLess/hels-devexp/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
