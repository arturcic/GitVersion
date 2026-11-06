package parity

import (
	"github.com/GitTools/GitVersion/go-prototype/internal/fixtures"
	"testing"
)

func TestFrozenRejectionContract(t *testing.T) {
	cp := fixtures.Checkpoint{ID: "rejected", Digest: "immutable"}
	good := Result{Before: "immutable", After: "immutable", ExitCode: 1, Error: "exit status 1"}
	if err := ValidateRejection(cp, "immutable", good); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"success", "exit-two", "timeout", "output", "mutation", "wrong-input", "missing-pin"} {
		t.Run(name, func(t *testing.T) {
			got := good
			digest := "immutable"
			switch name {
			case "success":
				got.ExitCode = 0
				got.Error = ""
			case "exit-two":
				got.ExitCode = 2
				got.Error = "exit status 2"
			case "timeout":
				got.Error = "context deadline exceeded"
			case "output":
				got.Stdout = []byte("{}")
			case "mutation":
				got.After = "changed"
			case "wrong-input":
				got.Before = ""
			case "missing-pin":
				digest = ""
			}
			if ValidateRejection(cp, digest, got) == nil {
				t.Fatal("invalid rejection accepted")
			}
		})
	}
	cp.Digest = "changed"
	if ValidateRejection(cp, "immutable", good) == nil {
		t.Fatal("changed fixture accepted")
	}
}
