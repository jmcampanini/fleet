package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// configPath reads the root --config flag and rejects an explicit empty path.
func configPath(cmd *cobra.Command) (string, error) {
	path, err := cmd.Flags().GetString("config")
	if err != nil {
		return "", err
	}
	if cmd.Flags().Changed("config") && path == "" {
		return "", fmt.Errorf("--config requires a nonempty file path")
	}
	return path, nil
}
