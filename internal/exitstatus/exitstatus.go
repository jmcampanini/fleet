// Package exitstatus classifies failures by the phase they interrupted.
package exitstatus

import "errors"

// Exit statuses by phase. Preflight covers everything before repository
// work starts; work covers failures whose report has already been written.
const (
	ExitSuccess   = 0
	ExitWork      = 1
	ExitPreflight = 2
)

// WorkError marks a failure raised after repository work started, so the
// process exits ExitWork rather than ExitPreflight.
type WorkError struct {
	Err error
}

func (e WorkError) Error() string { return e.Err.Error() }

func (e WorkError) Unwrap() error { return e.Err }

// ExitCode maps an error returned by the root command to a process exit
// status.
func ExitCode(err error) int {
	if err == nil {
		return ExitSuccess
	}
	var afterWork WorkError
	if errors.As(err, &afterWork) {
		return ExitWork
	}
	return ExitPreflight
}
