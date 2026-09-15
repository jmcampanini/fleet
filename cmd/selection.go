package cmd

import (
	"fmt"

	"github.com/jmcampanini/fleet/internal/config"
	"github.com/jmcampanini/fleet/internal/inventory"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// selection is the group flag and resolution step that every command
// operating on repositories shares.
type selection struct {
	groups []string
}

func (s *selection) bind(flags *pflag.FlagSet) {
	flags.StringArrayVar(&s.groups, "group", nil, "Select one group; repeat for several groups")
}

// resolve loads the configuration once and returns the selected repositories
// in complete-identity order. An empty selection is reported on stderr and
// returned as is; callers decide whether that is an error.
func (s selection) resolve(cmd *cobra.Command, refs []string) (config.Loaded, []inventory.Repository, error) {
	path, err := configPath(cmd)
	if err != nil {
		return config.Loaded{}, nil, err
	}
	loaded, err := config.Load(path, cmd.Root().PersistentFlags())
	if err != nil {
		return config.Loaded{}, nil, err
	}
	repos, err := loaded.Inventory.Select(refs, s.groups)
	if err != nil {
		return config.Loaded{}, nil, err
	}

	if len(repos) == 0 {
		if _, err := fmt.Fprintln(cmd.ErrOrStderr(), "No repositories selected."); err != nil {
			return config.Loaded{}, nil, err
		}
	}
	return loaded, repos, nil
}
