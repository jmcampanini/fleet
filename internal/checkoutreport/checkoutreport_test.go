package checkoutreport

import (
	"bytes"
	"testing"

	"github.com/jmcampanini/fleet/internal/checkout"
)

func TestWriteResultDescribesEachOutcomeOnOneLine(t *testing.T) {
	cases := []struct {
		name   string
		result checkout.Result
		want   string
	}{
		{
			name:   "current",
			result: checkout.Result{Branch: "main", Commit: "abc1234567", Status: checkout.StatusCurrent},
			want:   "\ueab2  one  current  abc1234 (main)\n",
		},
		{
			name: "updated on a long branch",
			result: checkout.Result{Branch: "release/next-version-更新", Commit: "def4567890", Status: checkout.StatusUpdated,
				Actions: []checkout.Action{{Kind: checkout.KindUpdate, Branch: "release/next-version-更新", From: "abc1234567", To: "def4567890"}}},
			want: "\uea77  one  updated  abc1234 → def4567 (release/next-version-更新)\n",
		},
		{
			name: "updated after a switch",
			result: checkout.Result{Branch: "main", Commit: "def4567890", Status: checkout.StatusUpdated,
				Actions: []checkout.Action{{Kind: checkout.KindSwitchBranch, Branch: "main", From: "feature", To: "main"}, {Kind: checkout.KindUpdate, Branch: "main", From: "abc1234567", To: "def4567890"}}},
			want: "\uea77  one  updated  switched from feature, updated abc1234 → def4567 (main)\n",
		},
		{
			name: "switched without a commit update",
			result: checkout.Result{Branch: "main", Commit: "abc1234567", Status: checkout.StatusUpdated,
				Actions: []checkout.Action{{Kind: checkout.KindSwitchBranch, Branch: "main", From: "feature", To: "main"}}},
			want: "\uea77  one  updated  switched from feature (main)\n",
		},
		{
			name: "cloned",
			result: checkout.Result{Branch: "main", Commit: "abc1234567", Status: checkout.StatusCloned,
				Actions: []checkout.Action{{Kind: checkout.KindClone, Branch: "main", To: "abc1234567"}}},
			want: "\uea77  one  cloned   abc1234 (main)\n",
		},
		{
			name:   "present",
			result: checkout.Result{Branch: "develop", Commit: "abc1234567", Status: checkout.StatusPresent},
			want:   "\ueab2  one  present  abc1234 (develop)\n",
		},
		{
			name: "planned sync with unresolved history",
			result: checkout.Result{Branch: "main", Commit: "def4567890", Status: checkout.StatusPlanned, HistoryUnresolved: true,
				PlannedActions: []checkout.Action{{Kind: checkout.KindCreateBranch, Branch: "main", To: "def4567890"}, {Kind: checkout.KindSwitchBranch, Branch: "main", From: "feature", To: "main"}}},
			want: "\uea70  one  planned  would create branch main, switch from feature; history unresolved (main)\n",
		},
		{
			name: "planned clone",
			result: checkout.Result{Branch: "main", Commit: "def4567890", Status: checkout.StatusPlanned,
				PlannedActions: []checkout.Action{{Kind: checkout.KindClone, Branch: "main", To: "def4567890"}}},
			want: "\uea70  one  planned  would clone at def4567 (main)\n",
		},
		{
			name:   "planned with nothing to do",
			result: checkout.Result{Branch: "main", Commit: "abc1234567", Status: checkout.StatusPlanned},
			want:   "\uea70  one  planned  up to date at abc1234 (main)\n",
		},
		{
			name:   "missing",
			result: checkout.Result{Status: checkout.StatusMissing},
			want:   "\uea6c  one  missing  run 'fleet clone one'\n",
		},
		{
			name: "failed after completed work with a multi-line error",
			result: checkout.Result{Branch: "main", Status: checkout.StatusFailed,
				Actions: []checkout.Action{{Kind: checkout.KindCreateBranch, Branch: "main", To: "abc1234567"}},
				Error:   "dirty working tree\nPreserve or resolve local work before rerunning.\n M file\n?? other"},
			want: "\uea87  one  failed   created branch main; dirty working tree (main)\n" +
				"    Preserve or resolve local work before rerunning.\n" +
				"     M file\n" +
				"    ?? other\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := WriteResult(&out, "one", 3, tc.result); err != nil {
				t.Fatal(err)
			}

			if out.String() != tc.want {
				t.Errorf("WriteResult(%+v) = %q, want %q", tc.result, out.String(), tc.want)
			}
		})
	}
}

func TestWriteSummaryCountsStatusesInWords(t *testing.T) {
	cases := []struct {
		name   string
		report Report
		want   string
	}{
		{
			name: "mixed sync",
			report: Report{Results: []checkout.Result{
				{Status: checkout.StatusCurrent}, {Status: checkout.StatusCurrent}, {Status: checkout.StatusUpdated}, {Status: checkout.StatusMissing}, {Status: checkout.StatusFailed},
			}},
			want: "\n5 repositories: 1 failed, 1 missing, 1 updated, 2 up to date\n",
		},
		{
			name: "mixed clone",
			report: Report{Results: []checkout.Result{
				{Status: checkout.StatusPresent}, {Status: checkout.StatusCloned}, {Status: checkout.StatusFailed},
			}},
			want: "\n3 repositories: 1 failed, 1 cloned, 1 present\n",
		},
		{
			name:   "dry run",
			report: Report{DryRun: true, Results: []checkout.Result{{Status: checkout.StatusPlanned}}},
			want:   "\n1 repository: 1 planned (dry run)\n",
		},
		{
			name:   "empty selection",
			report: Report{Complete: true, Results: []checkout.Result{}},
			want:   "0 repositories\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := WriteSummary(&out, tc.report); err != nil {
				t.Fatal(err)
			}

			if out.String() != tc.want {
				t.Errorf("WriteSummary(%+v) = %q, want %q", tc.report, out.String(), tc.want)
			}
		})
	}
}
