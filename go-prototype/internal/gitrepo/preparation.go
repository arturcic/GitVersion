package gitrepo

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	git "github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/storer"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/client"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/server"
)

// go-git's default file transport shells out to Git. Use its in-process
// transport with the same layout handling as version calculation.
func init() {
	client.InstallProtocol("file", server.NewClient(localRepositoryLoader{}))
}

type localRepositoryLoader struct{}

func (localRepositoryLoader) Load(endpoint *transport.Endpoint) (storer.Storer, error) {
	layout, err := openLayout(endpoint.Path)
	if errors.Is(err, errRepositoryNotFound) {
		return nil, transport.ErrRepositoryNotFound
	}
	if err != nil {
		return nil, err
	}
	repo, err := layout.open()
	if err != nil {
		return nil, err
	}
	return repo.Storer, nil
}

// Credentials are supplied to the transport without writing them to Git config.
type Credentials struct{ Username, Password string }

func (c Credentials) auth(source string) transport.AuthMethod {
	if c.Username == "" && c.Password == "" {
		return nil
	}
	if strings.HasPrefix(source, "https://") || strings.HasPrefix(source, "http://") {
		return &http.BasicAuth{Username: c.Username, Password: c.Password}
	}
	return nil
}

// PrepareClone reuses a checkout with the requested origin or publishes a
// complete clone in the first free destination.
func PrepareClone(ctx context.Context, source, base string, noCheckout bool, credentials Credentials) (string, error) {
	endpoint, err := transport.NewEndpoint(source)
	if err != nil {
		return "", errors.New("Invalid dynamic repository URL.")
	}
	name := strings.ReplaceAll(filepath.Base(strings.TrimRight(endpoint.Path, "/\\")), ".git", "")
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\") {
		return "", errors.New("Dynamic Git repositories require a repository name.")
	}
	if base == "" {
		base = os.TempDir()
	}
	base, err = filepath.Abs(base)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(base, 0755); err != nil {
		return "", err
	}
	var staging string
	for suffix := 0; ; suffix++ {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		destination := filepath.Join(base, name)
		if suffix != 0 {
			destination += fmt.Sprintf("_%d", suffix)
		}
		if info, err := os.Lstat(destination); err == nil {
			if info.IsDir() {
				for _, path := range []string{filepath.Join(destination, ".git"), destination} {
					if matchesClone(path, source) {
						return path, nil
					}
				}
			}
			continue
		} else if !os.IsNotExist(err) {
			return "", err
		}
		if staging == "" {
			staging, err = os.MkdirTemp(base, ".gitversion-clone-")
			if err != nil {
				return "", err
			}
			defer os.RemoveAll(staging)
			if _, err := git.PlainCloneContext(ctx, filepath.Join(staging, ".git"), false, &git.CloneOptions{
				URL: source, NoCheckout: noCheckout, Auth: credentials.auth(source),
			}); err != nil {
				return "", transportFailure("clone", err)
			}
		}
		// Reserve the name without replacing an existing directory, including
		// an empty one created by the caller.
		if err := os.Mkdir(destination, 0755); err != nil {
			if os.IsExist(err) {
				continue
			}
			return "", err
		}
		path := filepath.Join(destination, ".git")
		if err := os.Rename(filepath.Join(staging, ".git"), path); err != nil {
			_ = os.Remove(destination)
			return "", err
		}
		return path, nil
	}
}

func matchesClone(path, source string) bool {
	layout, err := openLayout(path)
	if err != nil || layout.workingDirectory == "" {
		return false
	}
	repo, err := layout.open()
	if err != nil {
		return false
	}
	remotes, err := repo.Remotes()
	if err != nil {
		return false
	}
	for _, remote := range remotes {
		for _, url := range remote.Config().URLs {
			if url == source {
				return true
			}
		}
	}
	return false
}

// PrepareRemote refreshes remote-tracking refs and returns the remote used for
// the calculation view. It leaves the checkout and local branch tips intact.
func PrepareRemote(ctx context.Context, path string, fetch, preferOrigin bool, credentials Credentials) (string, error) {
	layout, err := openLayout(path)
	if err != nil {
		return "", err
	}
	repo, err := layout.open()
	if err != nil {
		return "", err
	}
	remotes, err := repo.Remotes()
	if err != nil || len(remotes) == 0 {
		return "", err
	}
	remote := remotes[0]
	if len(remotes) > 1 {
		if !preferOrigin {
			return "", fmt.Errorf("%d remote(s) have been detected. When being run on a build server, the Git repository is expected to bear one (and no more than one) remote.", len(remotes))
		}
		for _, candidate := range remotes {
			if candidate.Config().Name == "origin" {
				remote = candidate
				break
			}
		}
	}
	cfg := remote.Config()
	if fetch {
		specs := append([]gitconfig.RefSpec(nil), cfg.Fetch...)
		hasBranches := false
		for _, spec := range specs {
			hasBranches = hasBranches || spec.Src() == "refs/heads/*"
		}
		if !hasBranches {
			specs = append(specs, gitconfig.RefSpec("+refs/heads/*:refs/remotes/"+cfg.Name+"/*"))
		}
		var auth transport.AuthMethod
		if len(cfg.URLs) > 0 {
			auth = credentials.auth(cfg.URLs[0])
		}
		if err := remote.FetchContext(ctx, &git.FetchOptions{RefSpecs: specs, Auth: auth}); err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
			return "", transportFailure("fetch", err)
		}
	}
	return cfg.Name, nil
}

func transportFailure(operation string, err error) error {
	switch {
	case errors.Is(err, transport.ErrAuthenticationRequired):
		return errors.New("Unauthorized: Incorrect username/password")
	case errors.Is(err, transport.ErrAuthorizationFailed):
		return errors.New("Forbidden: Possibly Incorrect username/password")
	case errors.Is(err, transport.ErrRepositoryNotFound):
		return errors.New("Not found: The repository was not found")
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return err
	default:
		return fmt.Errorf("Could not %s the Git repository.", operation)
	}
}
