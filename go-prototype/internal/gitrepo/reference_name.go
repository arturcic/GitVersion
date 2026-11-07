package gitrepo

import (
	"fmt"
	"strings"
)

type ReferenceName struct {
	Canonical, Friendly, WithoutOrigin, WithoutRemote   string
	IsLocalBranch, IsRemoteBranch, IsTag, IsPullRequest bool
}

func NewReferenceName(canonical string) ReferenceName {
	r := ReferenceName{Canonical: canonical, Friendly: canonical}
	r.IsLocalBranch = strings.HasPrefix(canonical, "refs/heads/")
	r.IsRemoteBranch = strings.HasPrefix(canonical, "refs/remotes/")
	r.IsTag = strings.HasPrefix(canonical, "refs/tags/")
	for _, prefix := range []string{"refs/pull/", "refs/pull-requests/", "refs/remotes/pull/", "refs/remotes/pull-requests/"} {
		r.IsPullRequest = r.IsPullRequest || strings.HasPrefix(canonical, prefix)
	}
	switch {
	case r.IsLocalBranch:
		r.Friendly = strings.TrimPrefix(canonical, "refs/heads/")
	case r.IsRemoteBranch:
		r.Friendly = strings.TrimPrefix(canonical, "refs/remotes/")
	case r.IsTag:
		r.Friendly = strings.TrimPrefix(canonical, "refs/tags/")
	}
	r.WithoutOrigin, r.WithoutRemote = r.Friendly, r.Friendly
	if r.IsRemoteBranch && !r.IsPullRequest {
		r.WithoutOrigin = strings.TrimPrefix(r.Friendly, "origin/")
		if _, name, ok := strings.Cut(r.Friendly, "/"); ok {
			r.WithoutRemote = name
		}
	}
	return r
}

func (r ReferenceName) EquivalentTo(name *string) bool {
	return name != nil && (strings.EqualFold(r.Canonical, *name) || strings.EqualFold(r.Friendly, *name) || strings.EqualFold(r.WithoutOrigin, *name))
}

type ReferenceArgumentError struct{ ParamName, Kind string }

func (e *ReferenceArgumentError) Error() string {
	return fmt.Sprintf("%s cannot be %s", e.ParamName, e.Kind)
}

// ReferenceFromTagName treats the supplied value literally, even if it already
// starts with a refs/ prefix, as the pinned C# FromTagName constructor does.
func ReferenceFromTagName(name *string) (ReferenceName, error) {
	if name == nil {
		return ReferenceName{}, &ReferenceArgumentError{"tagName", "null"}
	}
	if *name == "" {
		return ReferenceName{}, &ReferenceArgumentError{"tagName", "empty"}
	}
	return NewReferenceName("refs/tags/" + *name), nil
}
