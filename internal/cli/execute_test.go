package cli

import (
	"bytes"
	"testing"
)

func execute(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	root := NewRoot()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs(args)
	err := runRoot(t.Context(), root)
	return stdout.String(), stderr.String(), err
}
