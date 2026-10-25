// Fleet syncs primary Git checkouts and lists their issues and pull requests.
package main

import (
	"os"

	"github.com/jmcampanini/fleet/internal/cli"
	"github.com/jmcampanini/fleet/internal/exitstatus"
)

func main() {
	os.Exit(exitstatus.ExitCode(cli.Execute()))
}
