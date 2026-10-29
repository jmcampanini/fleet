package cli

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestHelpCompletesCommandPaths(t *testing.T) {
	root := NewRoot()
	help, _, err := root.Find([]string{"help"})
	if err != nil || help.ValidArgsFunction == nil {
		t.Fatalf("help completion = %v, want a completion function", err)
	}
	for _, tc := range []struct {
		args   []string
		prefix string
		want   string
	}{
		{prefix: "iss", want: "issues"},
		{prefix: "json", want: "json-reports"},
		{args: []string{"completion"}, prefix: "z", want: "zsh"},
		{args: []string{"unknown"}},
	} {
		completions, directive := help.ValidArgsFunction(help, tc.args, tc.prefix)
		var names []string
		for _, completion := range completions {
			name, _, _ := strings.Cut(completion, "\t")
			names = append(names, name)
		}
		if got := strings.Join(names, ","); got != tc.want || directive != cobra.ShellCompDirectiveNoFileComp {
			t.Errorf("complete help %v %q = %q, directive %v, want %q without files", tc.args, tc.prefix, got, directive, tc.want)
		}
	}
}
