// Fleet syncs primary Git checkouts and lists their issues and pull requests.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/jmcampanini/fleet/cmd"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := cmd.NewRoot().ExecuteContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "fleet: %s\n", err)
		os.Exit(cmd.ExitCode(err))
	}
}
