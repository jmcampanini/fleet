package cli

import (
	"fmt"

	"github.com/jmcampanini/fleet/internal/config"
	"github.com/spf13/cobra"
)

func loadConfig(cmd *cobra.Command) (config.Loaded, error) {
	path, err := cmd.Flags().GetString("config")
	if err != nil {
		return config.Loaded{}, err
	}
	if cmd.Flags().Changed("config") && path == "" {
		return config.Loaded{}, fmt.Errorf("--config requires a nonempty file path")
	}

	var flags config.Overrides
	if cmd.Flags().Changed("limit") {
		limit, err := cmd.Flags().GetInt("limit")
		if err != nil {
			return config.Loaded{}, err
		}
		switch cmd.Name() {
		case "issues":
			flags.IssuesLimit = &limit
		case "prs":
			flags.PRsLimit = &limit
		}
	}
	return config.Load(path, flags)
}
