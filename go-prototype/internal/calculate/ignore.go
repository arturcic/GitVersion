package calculate

import (
	"github.com/GitTools/GitVersion/go-prototype/internal/config"
	"github.com/GitTools/GitVersion/go-prototype/internal/gitrepo"
	"time"
)

type commitFilter struct {
	repository *gitrepo.Repository
	filters    []VersionFilter
	paths      bool
	cache      map[string]bool
}

func newCommitFilter(r *gitrepo.Repository, settings config.IgnoreSettings) (*commitFilter, error) {
	out := &commitFilter{repository: r, cache: map[string]bool{}}
	if len(settings.SHAs) > 0 {
		f, err := NewShaVersionFilter(settings.SHAs)
		if err != nil {
			return nil, err
		}
		out.filters = append(out.filters, f)
	}
	if settings.Before != nil {
		out.filters = append(out.filters, MinDateVersionFilter{Minimum: *settings.Before})
	}
	if len(settings.Paths) > 0 {
		f, err := NewPathFilter(settings.Paths)
		if err != nil {
			return nil, err
		}
		out.filters = append(out.filters, f)
		out.paths = true
	}
	return out, nil
}
func (f *commitFilter) include(id string) (bool, error) {
	if len(f.filters) == 0 || id == "" {
		return true, nil
	}
	if v, ok := f.cache[id]; ok {
		return v, nil
	}
	var c gitrepo.Commit
	var err error
	if f.paths {
		c, err = f.repository.Commit(id)
	} else {
		c, err = f.repository.CommitMetadata(id)
	}
	if err != nil {
		return false, err
	}
	for _, filter := range f.filters {
		excluded, _, err := filter.ExcludeCommit(&FilterCommit{SHA: id, When: time.Unix(c.When, 0), Paths: c.DiffPaths})
		if err != nil {
			return false, err
		}
		if excluded {
			f.cache[id] = false
			return false, nil
		}
	}
	f.cache[id] = true
	return true, nil
}
func (f *commitFilter) ids(ids []string) ([]string, error) {
	if len(f.filters) == 0 {
		return ids, nil
	}
	out := []string{}
	for _, id := range ids {
		ok, err := f.include(id)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, id)
		}
	}
	return out, nil
}
