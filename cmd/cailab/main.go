package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/msinclair25/cailab/internal/cli"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	cli.Version = version
	cli.Commit = commit
	cli.Date = date
	os.Exit(cli.New(os.Stdout, os.Stderr).Run(ctx, os.Args[1:]))
}
