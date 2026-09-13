package cmd

import (
	"testing"

	"github.com/jmcampanini/fleet/internal/checkout"
	"github.com/spf13/cobra"
)

func TestRenderCheckoutAlignsColumnsAndSummarizesStatuses(t *testing.T) {
	cases := []struct {
		name   string
		report checkoutReport
		want   string
	}{
		{
			name: "sync outcomes",
			report: checkoutReport{Results: []checkout.Result{
				{Repository: "github.com/a/one", Branch: "main", Commit: "abc1234567", Status: checkout.StatusCurrent},
				{Repository: "github.com/a/two-longer", Branch: "main", Commit: "def4567890", Status: checkout.StatusUpdated,
					Actions: []checkout.Action{{Kind: checkout.KindSwitchBranch, Branch: "main", From: "feature", To: "main"}, {Kind: checkout.KindUpdate, Branch: "main", From: "abc1234567", To: "def4567890"}}},
				{Repository: "github.com/a/three", Status: checkout.StatusMissing},
				{Repository: "github.com/a/four", Branch: "main", Commit: "abc1234567", Status: checkout.StatusFailed,
					Actions: []checkout.Action{{Kind: checkout.KindCreateBranch, Branch: "main", To: "abc1234567"}},
					Error:   "dirty working tree; commit, stash, or remove changes yourself before rerunning\n M file"},
			}},
			want: "current  github.com/a/one         main  abc1234\n" +
				"updated  github.com/a/two-longer  main  abc1234 -> def4567  switched from feature\n" +
				"missing  github.com/a/three                                 run 'fleet clone github.com/a/three'\n" +
				"failed   github.com/a/four        main  abc1234             created branch main; dirty working tree; commit, stash, or remove changes yourself before rerunning\n" +
				"4 repositories: 1 current, 1 updated, 1 missing, 1 failed\n",
		},
		{
			name: "dry run with unresolved history",
			report: checkoutReport{DryRun: true, Results: []checkout.Result{
				{Repository: "github.com/a/one", Branch: "main", Commit: "abc1234567", Status: checkout.StatusPlanned, HistoryUnresolved: true,
					PlannedActions: []checkout.Action{{Kind: checkout.KindSwitchBranch, Branch: "main", From: "feature", To: "main"}, {Kind: checkout.KindUpdate, Branch: "main", From: "abc1234567", To: "def4567890"}}},
				{Repository: "github.com/a/two", Branch: "main", Status: checkout.StatusPlanned,
					PlannedActions: []checkout.Action{{Kind: checkout.KindClone, Branch: "main", To: "def4567890"}}},
			}},
			want: "planned  github.com/a/one  main  abc1234 -> def4567  switch from feature, update; history unresolved\n" +
				"planned  github.com/a/two  main                      clone\n" +
				"2 repositories: 2 planned (dry run)\n",
		},
		{
			name:   "empty selection",
			report: checkoutReport{Complete: true, Results: []checkout.Result{}},
			want:   "0 repositories\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := render(t, func(command *cobra.Command) error { return renderCheckout(command, tc.report, false) })

			if out != tc.want {
				t.Errorf("renderCheckout output = %q, want %q", out, tc.want)
			}
		})
	}
}
