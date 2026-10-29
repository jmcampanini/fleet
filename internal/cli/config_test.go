package cli

import (
	"testing"

	"github.com/jmcampanini/fleet/internal/exitstatus"
)

func TestConfigOutputFailure(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := NewRoot()
	root.SetArgs([]string{"config", "--provenance"})
	root.SetOut(failingWriter{})

	err := root.ExecuteContext(t.Context())

	if got := exitstatus.ExitCode(err); got != exitstatus.ExitWork {
		t.Errorf("config with failed output = %v (exit %d), want exit 1", err, got)
	}
}
