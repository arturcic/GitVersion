package calculate

import (
	"fmt"
	"github.com/GitTools/GitVersion/go-prototype/internal/patterns"
	"github.com/GitTools/GitVersion/go-prototype/internal/semver"
	"strconv"
	"strings"
	"sync"
)

// MergeFormat is ordered: configured patterns run before the built-in formats.
type MergeFormat struct{ Name, Pattern string }
type MergeMessage struct {
	FormatName, TargetBranch, MergedBranch *string
	PullRequestNumber                      *int
	Version                                *semver.Version
}

func (m MergeMessage) IsMergedPullRequest() bool { return m.PullRequestNumber != nil }

// ParseMergeMessage preserves missing captures separately from empty captures.
func ParseMergeMessage(message *string, custom []MergeFormat) (MergeMessage, error) {
	var out MergeMessage
	if message == nil {
		return out, fmt.Errorf("mergeMessage cannot be null")
	}
	if *message == "" {
		return out, nil
	}
	names := []string{"Default", "SmartGit", "BitBucketPull", "BitBucketPullv7", "BitBucketCloudPull", "GitHubPull", "RemoteTracking", "AzureDevOpsPull"}
	for i := 0; i < len(custom)+len(names); i++ {
		var name string
		var p *patterns.Pattern
		var err error
		if i < len(custom) {
			name = custom[i].Name
			p, err = patterns.Compile(custom[i].Pattern, 0)
		} else {
			name = names[i-len(custom)]
			if name == "GitHubPull" {
				p, err = githubMergePattern()
			} else if name == "RemoteTracking" {
				p, err = remoteMergePattern()
			} else {
				p, err = patterns.Builtin("MergeMessage." + name + "MergeMessageRegex")
			}
		}
		if err != nil {
			return out, err
		}
		m, err := p.Find(*message)
		if err != nil {
			return out, err
		}
		if m == nil {
			continue
		}
		out.FormatName = &name
		source, _ := m.Group("SourceBranch")
		out.MergedBranch = &source
		if target, ok := m.Group("TargetBranch"); ok {
			out.TargetBranch = &target
		}
		if raw, ok := m.Group("PullRequestNumber"); ok {
			if n, e := strconv.ParseInt(raw, 10, 32); e == nil {
				x := int(n)
				out.PullRequestNumber = &x
			}
		}
		if v, ok := versionInBranch(strings.TrimPrefix(source, "origin/")); ok {
			out.Version = &v
		}
		return out, nil
	}
	return out, nil
}

var githubMergePattern = sync.OnceValues(func() (*patterns.Pattern, error) {
	return patterns.Compile(`^Merge pull request #(?<PullRequestNumber>\d+) (from|in) (?:[^\s\/]+\/)?(?<SourceBranch>[^\s]*)(?: into (?<TargetBranch>[^\s]*))*`, patterns.IgnoreCase)
})

var remoteMergePattern = sync.OnceValues(func() (*patterns.Pattern, error) {
	return patterns.Compile(`^Merge remote-tracking branch '(?<SourceBranch>[^\s]*)'(?: into (?<TargetBranch>[^\s]*))*`, patterns.IgnoreCase)
})
