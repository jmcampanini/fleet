package cmd

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/jmcampanini/fleet/internal/config"
	"github.com/spf13/cobra"
)

type groupResult struct {
	Name         string   `json:"name"`
	Repositories []string `json:"repositories"`
}

type groupsReport struct {
	Groups []groupResult `json:"groups"`
}

func newGroups() *cobra.Command {
	var groups []string
	var jsonOutput bool
	command := &cobra.Command{
		Use: "groups", Short: "List configured groups and their repository members",
		Long: `List group names with their resolved server/org/repo members underneath.
Groups sort alphabetically; members sort by complete repository identity.
Each repository appears once within a group and under every group it
belongs to. Empty groups show (empty).

With no --group flags, list all configured groups. Each --group takes one
exact group name; repeat it to select several groups. Repeated names show
one section. Positional arguments and comma-separated names are not
supported. Invalid configuration or an unknown group fails before output.

Results go to stdout; diagnostics go to stderr. --json emits one object
containing a groups array; see 'fleet json-reports' for its fields.
No configured groups succeeds with 'No groups configured.' on stderr and
empty stdout, or {"groups":[]} with --json. Success exits 0; errors exit 2.
This command needs no CODE_DIR, Git, gh, local checkout, or network.

` + configHelp,
		Example: `  fleet groups
  fleet groups --group agents --group clis
  fleet groups --group agents --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			path, err := configPath(cmd)
			if err != nil {
				return err
			}
			loaded, err := config.Load(path, cmd.Root().PersistentFlags())
			if err != nil {
				return err
			}

			names := slices.Clone(groups)
			if len(names) == 0 {
				names = loaded.Inventory.GroupNames()
			}
			slices.Sort(names)
			names = slices.Compact(names)
			report := groupsReport{Groups: []groupResult{}}
			for _, name := range names {
				repos, err := loaded.Inventory.Select(nil, []string{name})
				if err != nil {
					return err
				}
				group := groupResult{Name: name, Repositories: []string{}}
				for _, repo := range repos {
					group.Repositories = append(group.Repositories, repo.ID)
				}
				report.Groups = append(report.Groups, group)
			}

			return renderGroups(cmd, report, jsonOutput)
		},
	}
	command.Flags().StringArrayVar(&groups, "group", nil, "Select one group; repeat for several groups")
	command.Flags().BoolVar(&jsonOutput, "json", false, "Emit a JSON report")
	return command
}

func renderGroups(cmd *cobra.Command, report groupsReport, jsonOutput bool) error {
	if jsonOutput {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(report)
	}
	if len(report.Groups) == 0 {
		_, err := fmt.Fprintln(cmd.ErrOrStderr(), "No groups configured.")
		return err
	}

	var out strings.Builder
	for index, group := range report.Groups {
		if index > 0 {
			out.WriteByte('\n')
		}
		fmt.Fprintln(&out, group.Name)
		for _, repo := range group.Repositories {
			fmt.Fprintf(&out, "  %s\n", repo)
		}
		if len(group.Repositories) == 0 {
			fmt.Fprintln(&out, "  (empty)")
		}
	}
	_, err := fmt.Fprint(cmd.OutOrStdout(), out.String())
	return err
}
