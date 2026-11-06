package parity

import (
	"fmt"
	"github.com/GitTools/GitVersion/go-prototype/internal/fixtures"
)

// ValidateRejection binds a frozen rejection to an exact immutable repository.
// Timeouts, missing executables, output, mutation and a different exit code are
// failures, never evidence of equivalent rejection. Diagnostics are a CLI gate.
func ValidateRejection(cp fixtures.Checkpoint, digest string, r Result) error {
	if digest == "" || cp.Digest != digest || r.Before == "" || r.After != r.Before || r.ExitCode != 1 || r.Error != "exit status 1" || len(r.Stdout) != 0 {
		return fmt.Errorf("rejection contract differs for %s", cp.ID)
	}
	return nil
}
