package gitrepo

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	git "github.com/go-git/go-git/v5"
)

func (r *Repository) detachedBranch(directory string, options OpenOptions) (string, error) {
	refs, err := r.References()
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
	localNames := map[string]bool{}
	for _, ref := range refs {
		name := NewReferenceName(ref.Name)
		if name.IsLocalBranch {
			localNames[name.Friendly] = true
			if ref.SHA == r.head {
				local = append(local, name)
			}
		}
	}
	for _, ref := range refs {
		name := NewReferenceName(ref.Name)
		if ref.SHA == r.head && name.IsRemoteBranch && !name.IsPullRequest &&
			strings.HasPrefix(name.Friendly, remoteName+"/") && name.WithoutRemote != "HEAD" &&
			!localNames[name.WithoutRemote] {
			local = append(local, NewReferenceName("refs/heads/"+name.WithoutRemote))
		}
	}
	for _, name := range local {
		if name.EquivalentTo(&options.BuildBranch) {
			return name.Canonical, nil
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

	pullBranches := map[string]bool{}
	add := func(canonical, sha string) {
		if sha != r.head {
			return
		}
		name := pullRequestBranchName(canonical)
		if name != "" {
			pullBranches[name] = true
		}
	}
	localRefs, err := (referenceStore{git: r.git, commonDirectory: r.dir}).list("refs/")
	if err != nil {
		return "", err
	}
	for _, ref := range localRefs {
		add(ref.Name().String(), ref.Hash().String())
	}
	remote, err := localOrigin(r.git, directory)
	if err != nil {
		return "", err
	}
	var advertisedTag bool
	if remote != nil {
		remoteRefs, err := (referenceStore{git: remote}).list("refs/")
		if err != nil {
			return "", err
		}
		for _, ref := range remoteRefs {
			add(ref.Name().String(), ref.Hash().String())
			if ref.Name().IsTag() {
				for _, tag := range tags {
					if tag.Name == ref.Name().String() && tag.Commit == r.head {
						for _, localRef := range refs {
							if localRef.Name == tag.Name && localRef.SHA == ref.Hash().String() {
								advertisedTag = true
							}
						}
					}
				}
			}
		}
	}

	if build := pullRequestBranchName(options.BuildBranch); build != "" && pullBranches[build] {
		return build, nil
	}
	if branch, ok := strings.CutPrefix(strings.ToLower(options.BuildBranch), "pr-"); ok && branch != "" && len(pullBranches) == 1 {
		for name := range pullBranches {
			parts := strings.Split(name, "/")
			if len(parts) >= 4 && parts[3] == branch {
				return "refs/heads/" + options.BuildBranch, nil
			}
		}
	}
	if len(pullBranches) == 0 && advertisedTag {
		return "(no branch)", nil
	}
	if len(pullBranches) == 1 {
		for branch := range pullBranches {
			return branch, nil
		}
	}
	if len(pullBranches) > 1 {
		return "", fmt.Errorf("multiple pull request refs point at detached HEAD %s", r.head)
	}
	return "", fmt.Errorf("unsupported detached HEAD")
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

func localOrigin(repo *git.Repository, directory string) (*git.Repository, error) {
	remotes, err := repo.Remotes()
	if err != nil || len(remotes) != 1 {
		return nil, err
	}
	urls := remotes[0].Config().URLs
	if len(urls) != 1 {
		return nil, nil
	}
	path := urls[0]
	if strings.HasPrefix(path, "file://") {
		parsed, err := url.Parse(path)
		if err != nil || (parsed.Host != "" && parsed.Host != "localhost") {
			return nil, err
		}
		path = parsed.Path
	} else if strings.Contains(path, ":") {
		return nil, nil
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(directory, path)
	}
	remote, err := git.PlainOpen(path)
	if err == git.ErrRepositoryNotExists {
		return nil, nil
	}
	return remote, err
}
