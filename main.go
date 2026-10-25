// Fleet syncs primary Git checkouts and lists their issues and pull requests.
package main

import (
	"os"

	"github.com/jmcampanini/fleet/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
