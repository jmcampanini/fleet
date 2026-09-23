package process

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNoninteractiveEnvironmentAndClosedStdin(t *testing.T) {
	t.Setenv("GIT_DIR", filepath.Join(t.TempDir(), "wrong-repository"))
	t.Setenv("GH_DEBUG", "api")
	output, err := Execute(t.Context(), "", "sh", "-c", `if read line; then exit 2; fi; printf '%s|%s|%s|%s|%s|%s' "$GIT_TERMINAL_PROMPT" "$GH_PROMPT_DISABLED" "$GIT_SSH_COMMAND" "$GIT_DIR" "$GH_DEBUG" "$GCM_INTERACTIVE"`)
	if err != nil {
		t.Fatal(err)
	}
	if output != "0|1|ssh -oBatchMode=yes -oConnectTimeout=15|||Never" {
		t.Errorf("child environment = %q", output)
	}
}

func TestSuccessfulOutputPreservesSpacesAndTabs(t *testing.T) {
	cases := []struct {
		name   string
		output string
		want   string
	}{
		{name: "status columns", output: " M first\n M second\n", want: " M first\n M second"},
		{name: "spaces and tabs", output: " \tvalue\t \n", want: " \tvalue\t "},
		{name: "CRLF", output: "value\r\n", want: "value"},
		{name: "empty", output: "", want: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			output, err := Execute(t.Context(), "", "sh", "-c", `printf '%s' "$1"`, "sh", tc.output)
			if err != nil {
				t.Fatal(err)
			}

			if output != tc.want {
				t.Errorf("Execute() = %q, want %q", output, tc.want)
			}
		})
	}
}

func TestGitSafetyConfiguration(t *testing.T) {
	for name, want := range map[string]string{"core.hooksPath": "/dev/null", "submodule.recurse": "false", "maintenance.auto": "false", "gc.auto": "0"} {
		output, err := Execute(t.Context(), t.TempDir(), "git", "config", "--get", name)
		if err != nil || output != want {
			t.Errorf("Git config %q = %q, %v, want %q", name, output, err, want)
		}
	}
}

func TestCancellationStopsSubprocessGroup(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := Execute(ctx, "", "sh", "-c", "sleep 20 & wait")
	if !errors.Is(err, context.DeadlineExceeded) || time.Since(start) > 3*time.Second {
		t.Fatalf("cancellation took %s with error %v", time.Since(start), err)
	}
}

func TestErrorsContainStderr(t *testing.T) {
	_, err := Execute(t.Context(), "", "sh", "-c", "printf 'remote denied' >&2; exit 7")
	if err == nil || !strings.Contains(err.Error(), "remote denied") {
		t.Fatalf("error = %v", err)
	}
}
