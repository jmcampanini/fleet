package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestGroupsListing(t *testing.T) {
	t.Setenv("FLEET_ISSUES_LIMIT", "15")
	t.Setenv("FLEET_PRS_LIMIT", "15")
	path := filepath.Join(t.TempDir(), "fleet.toml")
	if err := os.WriteFile(path, []byte(`[repos."github.com/example/gibson"]
[repos."github.com/example/molly"]
[repos."github.com/example/fleet"]
[groups]
scratch = []
clis = ["gibson", "fleet"]
agents = ["molly", "gibson", "example/gibson"]
`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CODE_DIR", "")
	t.Setenv("PATH", "")
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "all text",
			want: "agents\n  github.com/example/gibson\n  github.com/example/molly\n\nclis\n  github.com/example/fleet\n  github.com/example/gibson\n\nscratch\n  (empty)\n",
		},
		{
			name: "filtered sorted and deduplicated",
			args: []string{"--group", "clis", "--group", "agents", "--group", "clis"},
			want: "agents\n  github.com/example/gibson\n  github.com/example/molly\n\nclis\n  github.com/example/fleet\n  github.com/example/gibson\n",
		},
		{
			name: "empty group",
			args: []string{"--group", "scratch"},
			want: "scratch\n  (empty)\n",
		},
		{
			name: "all JSON",
			args: []string{"--json"},
			want: `{"groups":[{"name":"agents","repositories":["github.com/example/gibson","github.com/example/molly"]},{"name":"clis","repositories":["github.com/example/fleet","github.com/example/gibson"]},{"name":"scratch","repositories":[]}]}`,
		},
		{
			name: "filtered JSON",
			args: []string{"--group", "scratch", "--json"},
			want: `{"groups":[{"name":"scratch","repositories":[]}]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := append([]string{"groups", "--config", path}, tt.args...)
			out, stderr, err := execute(t, args...)
			if err != nil || stderr != "" {
				t.Fatalf("groups = %q, stderr %q, error %v", out, stderr, err)
			}
			if strings.HasPrefix(tt.want, "{") {
				var got, want any
				if err := json.Unmarshal([]byte(out), &got); err != nil {
					t.Fatalf("groups JSON = %q: %v", out, err)
				}
				if err := json.Unmarshal([]byte(tt.want), &want); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(got, want) {
					t.Errorf("groups JSON = %s, want %s", out, tt.want)
				}
				return
			}
			if out != tt.want {
				t.Errorf("groups = %q, want %q", out, tt.want)
			}
		})
	}
}

func TestGroupsWithoutConfiguredGroups(t *testing.T) {
	t.Setenv("FLEET_ISSUES_LIMIT", "15")
	t.Setenv("FLEET_PRS_LIMIT", "15")
	path := filepath.Join(t.TempDir(), "fleet.toml")
	if err := os.WriteFile(path, []byte(`[repos."github.com/example/fleet"]`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CODE_DIR", "")
	t.Setenv("PATH", "")
	for _, tt := range []struct {
		name   string
		args   []string
		stdout string
		stderr string
	}{
		{name: "text", stderr: "No groups configured.\n"},
		{name: "JSON", args: []string{"--json"}, stdout: "{\"groups\":[]}\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			out, stderr, err := execute(t, append([]string{"groups", "--config", path}, tt.args...)...)
			if err != nil || out != tt.stdout || stderr != tt.stderr {
				t.Errorf("groups = %q, %q, %v; want %q, %q, nil", out, stderr, err, tt.stdout, tt.stderr)
			}
		})
	}
}

func TestGroupsErrorsLeaveStdoutEmpty(t *testing.T) {
	t.Setenv("FLEET_ISSUES_LIMIT", "15")
	t.Setenv("FLEET_PRS_LIMIT", "15")
	t.Setenv("PATH", "")
	for _, tt := range []struct {
		name   string
		config string
		groups []string
		want   string
	}{
		{name: "unknown after valid", config: "[groups]\nagents = []\n", groups: []string{"agents", "missing"}, want: `unknown group "missing"`},
		{name: "comma separated", config: "[groups]\nagents = []\nclis = []\n", groups: []string{"agents,clis"}, want: `unknown group "agents,clis"`},
		{name: "case sensitive", config: "[groups]\nagents = []\n", groups: []string{"Agents"}, want: `unknown group "Agents"`},
		{name: "empty filter", config: "[groups]\nagents = []\n", groups: []string{""}, want: `unknown group ""`},
		{name: "invalid unselected group", config: "[groups]\nagents = []\nbroken = [\"missing\"]\n", groups: []string{"agents"}, want: `group "broken"`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "fleet.toml")
			if err := os.WriteFile(path, []byte(tt.config), 0o600); err != nil {
				t.Fatal(err)
			}
			args := []string{"groups", "--config", path}
			for _, group := range tt.groups {
				args = append(args, "--group", group)
			}
			for _, format := range [][]string{nil, {"--json"}} {
				out, _, err := execute(t, append(args, format...)...)
				if err == nil || !strings.Contains(err.Error(), tt.want) || out != "" {
					t.Errorf("groups %v = %q, %v; want empty stdout and error containing %q", format, out, err, tt.want)
				}
			}
		})
	}
}
