package gitrepo

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/GitTools/GitVersion/go-prototype/internal/patterns"
	git "github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
)

type ignoreRule struct {
	pattern                *patterns.Pattern
	directoryOnly, negated bool
}
type ignoreSource struct {
	base  string
	rules []ignoreRule
}

// parseIgnoreRules mirrors the pinned managed GitIgnoreRules parser. Rules match
// an entry itself; directory exclusion is handled by not descending into it.
// This matters for !build/artifacts followed by a separately ignored obj child.
func parseIgnoreRules(text string, ignoreCase bool) ([]ignoreRule, error) {
	var rules []ignoreRule
	for _, line := range strings.Split(text, "\n") {
		p := strings.TrimSuffix(line, "\r")
		for strings.HasSuffix(p, " ") && !strings.HasSuffix(p, "\\ ") {
			p = p[:len(p)-1]
		}
		if p == "" || p[0] == '#' {
			continue
		}
		negated := p[0] == '!'
		if negated {
			p = p[1:]
		}
		directory := strings.HasSuffix(p, "/")
		if directory {
			p = p[:len(p)-1]
		}
		if p == "" {
			continue
		}
		if !strings.Contains(p, "/") {
			p = "**/" + p
		}
		p = strings.TrimLeft(p, "/")
		var out strings.Builder
		out.WriteByte('^')
		for i := 0; i < len(p); {
			segment := i == 0 || p[i-1] == '/'
			if segment && strings.HasPrefix(p[i:], "**/") {
				out.WriteString("(?:[^/]+/)*")
				i += 3
				continue
			}
			if segment && p[i:] == "**" {
				out.WriteString(".*")
				i += 2
				continue
			}
			switch p[i] {
			case '*':
				out.WriteString("[^/]*")
				i++
				if i < len(p) && p[i] == '*' {
					i++
				}
			case '?':
				out.WriteString("[^/]")
				i++
			case '[':
				end := strings.IndexByte(p[i+1:], ']')
				if end < 0 {
					out.WriteString(`\[`)
					i++
					continue
				}
				end += i + 1
				body := p[i+1 : end]
				if strings.HasPrefix(body, "!") {
					body = "^" + body[1:]
				}
				out.WriteByte('[')
				out.WriteString(body)
				out.WriteByte(']')
				i = end + 1
			case '\\':
				if i+1 < len(p) {
					i++
				}
				start := i
				i++
				for i < len(p) && p[i]&0xc0 == 0x80 {
					i++
				}
				out.WriteString(regexp.QuoteMeta(p[start:i]))
			default:
				// Copy complete UTF-8 literals; patterns.Compile translates them to the
				// UTF-16 matching model used by the reference engine.
				start := i
				i++
				for i < len(p) && p[i]&0xc0 == 0x80 {
					i++
				}
				out.WriteString(regexp.QuoteMeta(p[start:i]))
			}
		}
		out.WriteByte('$')
		options := patterns.Options(0)
		if ignoreCase {
			options = patterns.IgnoreCase
		}
		regex, err := patterns.Compile(out.String(), options)
		if err != nil {
			return nil, err
		}
		rules = append(rules, ignoreRule{regex, directory, negated})
	}
	return rules, nil
}
func (s ignoreSource) decision(path string, directory bool) (bool, bool, error) {
	path = strings.TrimPrefix(path, s.base)
	for i := len(s.rules) - 1; i >= 0; i-- {
		rule := s.rules[i]
		if rule.directoryOnly && !directory {
			continue
		}
		m, err := rule.pattern.Find(path)
		if err != nil {
			return false, false, err
		}
		if m != nil {
			return !rule.negated, true, nil
		}
	}
	return false, false, nil
}
func loadIgnoreSource(path, base string, ignoreCase bool) (ignoreSource, error) {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return ignoreSource{base: base}, nil
	}
	if err != nil {
		return ignoreSource{}, err
	}
	rules, err := parseIgnoreRules(string(raw), ignoreCase)
	return ignoreSource{base, rules}, err
}
func gitConfigValue(path, section, subsection, key string) (string, bool, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	defer f.Close()
	cfg, err := gitconfig.ReadConfig(f)
	if err != nil {
		return "", false, err
	}
	if !cfg.Raw.HasSection(section) {
		return "", false, nil
	}
	s := cfg.Raw.Section(section)
	if subsection != "" {
		if !s.HasSubsection(subsection) {
			return "", false, nil
		}
		sub := s.Subsection(subsection)
		return sub.Option(key), sub.HasOption(key), nil
	}
	return s.Option(key), s.HasOption(key), nil
}

// reconcileUntracked uses the reference's ordered ignore sources and directory
// traversal. Tracked changes from go-git are retained regardless of ignore rules.
func reconcileUntracked(path, gitdir string, g *git.Repository, status git.Status) error {
	ignore, _, err := gitConfigValue(filepath.Join(gitdir, "config"), "core", "", "ignorecase")
	if err != nil {
		return err
	}
	ignoreCase := strings.EqualFold(ignore, "true") || strings.EqualFold(ignore, "yes") || ignore == "1" || strings.EqualFold(ignore, "on")
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	xdg := os.Getenv("XDG_CONFIG_HOME")
	if xdg == "" {
		xdg = filepath.Join(home, ".config")
	}
	global := filepath.Join(xdg, "git", "ignore")
	for _, p := range []string{filepath.Join(gitdir, "config"), filepath.Join(home, ".gitconfig"), filepath.Join(xdg, "git", "config")} {
		value, found, err := gitConfigValue(p, "core", "", "excludesfile")
		if err != nil {
			return err
		}
		if found {
			global = value
			break
		}
	}
	if strings.HasPrefix(global, "~/") {
		global = filepath.Join(home, global[2:])
	}
	var sources []ignoreSource
	for _, p := range []string{global, filepath.Join(gitdir, "info", "exclude")} {
		s, err := loadIgnoreSource(p, "", ignoreCase)
		if err != nil {
			return err
		}
		sources = append(sources, s)
	}
	index, err := g.Storer.Index()
	if err != nil {
		return err
	}
	tracked := map[string]bool{}
	for _, entry := range index.Entries {
		tracked[entry.Name] = true
	}
	for name, s := range status {
		if s.Staging == git.Untracked && s.Worktree == git.Untracked {
			delete(status, name)
		}
	}
	var walk func(string, string, []ignoreSource) error
	walk = func(dir, base string, sources []ignoreSource) error {
		source, err := loadIgnoreSource(filepath.Join(dir, ".gitignore"), base, ignoreCase)
		if err != nil {
			return err
		}
		// Do not share an append backing array with sibling directory scopes.
		sources = append(append([]ignoreSource{}, sources...), source)
		entries, err := os.ReadDir(dir)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if base == "" && entry.Name() == ".git" {
				continue
			}
			name := base + entry.Name()
			directory := entry.IsDir()
			ignored := false
			for i := len(sources) - 1; i >= 0; i-- {
				decision, found, err := sources[i].decision(name, directory)
				if err != nil {
					return fmt.Errorf("ignore matching %s: %w", name, err)
				}
				if found {
					ignored = decision
					break
				}
			}
			if ignored {
				continue
			}
			if directory {
				if err := walk(filepath.Join(dir, entry.Name()), name+"/", sources); err != nil {
					return err
				}
			} else if !tracked[name] {
				status[name] = &git.FileStatus{Staging: git.Untracked, Worktree: git.Untracked}
			}
		}
		return nil
	}
	return walk(path, "", sources)
}
