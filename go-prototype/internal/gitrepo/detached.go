package gitrepo

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-git/go-git/v5/plumbing"
)

func (r *Repository) detachedBranch(directory string, options OpenOptions) (string, error) {
	refs, err := r.References()
	if err != nil {
		return "", err
	}
	refs, err = r.NormalizedReferences(refs, options.IgnoreBranch)
	if err != nil {
		return "", err
	}
	if tag, ok := strings.CutPrefix(options.BuildBranch, "ref/tags/"); ok {
		tags, err := r.Tags(refs)
		if err != nil {
			return "", err
		}
		for _, candidate := range tags {
			if candidate.Name == "refs/tags/"+tag && candidate.Commit == r.head {
				return "refs/heads/" + options.BuildBranch, nil
			}
		}
	}
	remoteName := options.NormalizationRemote
	if remoteName == "" {
		remoteName = "origin"
	}
	var local []ReferenceName
	for _, ref := range refs {
		if ref.SHA == r.head && strings.HasPrefix(ref.Name, "refs/heads/") {
			local = append(local, NewReferenceName(ref.Name))
		}
	}
	if len(local) == 1 {
		return local[0].Canonical, nil
	}
	if len(local) > 1 {
		var simple []ReferenceName
		for _, branch := range local {
			if branch.Friendly == "main" {
				return branch.Canonical, nil
			}
			if !strings.ContainsAny(branch.Friendly, "/-") {
				simple = append(simple, branch)
			}
		}
		if len(simple) == 1 {
			return simple[0].Canonical, nil
		}
	}
	tags, err := r.Tags(refs)
	if err != nil {
		return "", err
	}
	for _, tag := range tags {
		if tag.Commit == r.head && (len(local) > 1 || options.BuildTag == tag.Name || options.BuildBranch == tag.Name) {
			return "(no branch)", nil
		}
	}
	if len(local) > 1 {
		return "", fmt.Errorf("Failed to try and guess branch to use. Move one of the branches along a commit to remove warning")
	}

	if options.BuildTag != "" {
		for _, tag := range tags {
			if tag.Name == options.BuildTag && tag.Commit == r.head {
				return "(no branch)", nil
			}
		}
	}
	remote, err := r.git.Remote(remoteName)
	if err != nil && options.NormalizationRemote == "" {
		remote, err = normalizationRemote(r.git, false)
	}
	if err != nil {
		return "", err
	}
	ctx := options.PreparationContext
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
	}
	remote = resolveFileRemote(r.git, remote, directory)
	tips, err := advertisedTips(ctx, remote, plumbing.NewHash(r.head), options.RemoteCredentials)
	if err != nil {
		return "", err
	}
	switch len(tips) {
	case 0:
		return "", fmt.Errorf("Couldn't find any remote tips pointing at the commit '%s'.", r.head)
	case 1:
		tip := tips[0]
		if strings.HasPrefix(tip, "refs/tags/") {
			return "(no branch)", nil
		}
		if branch := pullRequestBranchName(tip); branch != "" {
			return branch, nil
		}
		return "", fmt.Errorf("Remote tip '%s' doesn't look like a valid pull request.", tip)
	default:
		return "", fmt.Errorf("Found more than one remote tip pointing at the commit '%s'. Unable to determine which one to use (%s).", r.head, strings.Join(tips, ", "))
	}
}

func pullRequestBranchName(name string) string {
	name = strings.TrimPrefix(name, "refs/")
	for _, prefix := range []string{"heads/", "remotes/", "origin/"} {
		name = strings.TrimPrefix(name, prefix)
	}
	if strings.HasPrefix(name, "pull/") || strings.HasPrefix(name, "pull-requests/") || strings.HasPrefix(name, "merge-requests/") {
		return "refs/heads/" + name
	}
	return ""
}
