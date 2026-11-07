package main

import (
	"fmt"
	"strings"

	"github.com/GitTools/GitVersion/go-prototype/internal/gitrepo"
)

func branchSelection(explicit string, environment func(string) string) (target, contextual string, err error) {
	if strings.TrimSpace(explicit) != "" {
		return explicit, "", nil
	}
	for _, variable := range []string{"GIT_BRANCH", "Git_Branch"} {
		input := environment(variable)
		if strings.TrimSpace(input) == "" {
			continue
		}
		ref := gitrepo.ReferenceFromBranchName(input)
		name := ref.Friendly
		if ref.IsRemoteBranch {
			name = ref.WithoutRemote
		}
		if ref.IsTag || ref.IsPullRequest || (ref.IsRemoteBranch && !strings.Contains(ref.Friendly, "/")) ||
			!validContextBranch(input) || !validContextBranch(name) {
			return "", "", fmt.Errorf("Invalid branch name '%s' in GIT_BRANCH/Git_Branch.", input)
		}
		return "", "refs/heads/" + name, nil
	}
	return "", "", nil
}

// Match GitVersion's branch-name validation without expanding checkout syntax.
func validContextBranch(name string) bool {
	if name == "" || name == "HEAD" || name[0] == '-' || strings.HasSuffix(name, ".") ||
		strings.Contains(name, "..") || strings.Contains(name, "@{") ||
		strings.ContainsAny(name, "~^:?*[\\") ||
		strings.ContainsFunc(name, func(c rune) bool { return c <= ' ' || c == 0x7f }) {
		return false
	}
	for component := range strings.SplitSeq(name, "/") {
		if component == "" || component[0] == '.' || strings.HasSuffix(component, ".lock") {
			return false
		}
	}
	return true
}
