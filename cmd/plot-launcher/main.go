package main

import (
	"context"
	"fmt"
	"os"

	"github.com/PlotAI/plot-launcher/internal/cli"
)

func main() {
	runner, err := cli.NewRunner(os.Stdout, os.Stderr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "plot-launcher: %v\n", err)
		os.Exit(1)
	}
	os.Exit(runner.Run(context.Background(), os.Args[1:]))
}
