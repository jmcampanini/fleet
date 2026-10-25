package cli

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
)

func render(t *testing.T, write func(*cobra.Command) error) string {
	t.Helper()
	var out bytes.Buffer
	command := &cobra.Command{}
	command.SetOut(&out)
	if err := write(command); err != nil {
		t.Fatal(err)
	}
	return out.String()
}
