package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/DoNotResuscitate/nameforge/internal/cli"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	code := cli.RunContext(ctx, os.Args[1:], os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}
