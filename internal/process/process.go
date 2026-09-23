// Package process runs bounded, noninteractive Git and GitHub commands.
package process

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

// Run is the subprocess boundary used by repository operations.
type Run func(ctx context.Context, dir, program string, args ...string) (string, error)

// Execute runs a command with closed stdin and a 120-second deadline.
// Successful stdout retains spaces and tabs, but not trailing line endings.
func Execute(ctx context.Context, dir, program string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	if program == "git" {
		args = append([]string{"--no-optional-locks", "-c", "core.hooksPath=/dev/null", "-c", "submodule.recurse=false", "-c", "maintenance.auto=false", "-c", "gc.auto=0"}, args...)
	}
	command := exec.CommandContext(ctx, program, args...)
	command.Dir = dir
	command.WaitDelay = time.Second
	// Stop transport helpers and filters with their parent on cancellation.
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	command.Cancel = func() error {
		if err := syscall.Kill(-command.Process.Pid, syscall.SIGKILL); err != nil {
			return err
		}
		return nil
	}
	// Inherited Git routing variables must not redirect work to another checkout.
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(name, "GIT_") || name == "GH_DEBUG" || name == "GH_FORCE_TTY" {
			continue
		}
		command.Env = append(command.Env, entry)
	}
	command.Env = append(command.Env, "GIT_TERMINAL_PROMPT=0", "GCM_INTERACTIVE=Never", "GIT_SSH_COMMAND=ssh -oBatchMode=yes -oConnectTimeout=15", "GH_PROMPT_DISABLED=1", "GH_PAGER=cat", "NO_COLOR=1")
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Run(); err != nil {
		if ctx.Err() != nil {
			return "", fmt.Errorf("%s command interrupted or timed out: %w", program, ctx.Err())
		}
		return "", fmt.Errorf("%s: %w: %s", program, err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimRight(stdout.String(), "\r\n"), nil
}
