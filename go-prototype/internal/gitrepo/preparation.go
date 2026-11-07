package gitrepo

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"

	git "github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/client"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/server"
	"golang.org/x/crypto/ssh/knownhosts"
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

type connection struct {
	auth     transport.AuthMethod
	caBundle []byte
	socket   net.Conn
}

func (c connection) close() {
	if c.socket != nil {
		_ = c.socket.Close()
	}
}

func connectionOptions(ctx context.Context, source string, credentials Credentials) (connection, error) {
	endpoint, err := transport.NewEndpoint(source)
	if err != nil {
		return connection{}, nil
	}
	if endpoint.Protocol == "ssh" {
		return sshConnectionOptions(ctx, endpoint)
	}
	var options connection
	if endpoint.Protocol != "http" && endpoint.Protocol != "https" {
		return options, nil
	}
	if strings.TrimSpace(credentials.Username) != "" {
		options.auth = &http.BasicAuth{Username: credentials.Username, Password: credentials.Password}
	}
	if caPath := os.Getenv("GIT_SSL_CAINFO"); endpoint.Protocol == "https" && caPath != "" {
		options.caBundle, err = os.ReadFile(caPath)
		if err != nil {
			return connection{}, fmt.Errorf("read Git HTTPS CA bundle: %w", err)
		}
	}
	return options, nil
}

// PrepareClone reuses a checkout with the requested origin or publishes a
// complete clone in the first free destination.
func PrepareClone(ctx context.Context, source, base string, noCheckout bool, credentials Credentials) (string, error) {
	endpoint, err := transport.NewEndpoint(source)
	if err != nil {
		return "", errors.New("Invalid dynamic repository URL.")
	}
	if endpoint.Protocol == "file" && !strings.Contains(source, "://") {
		source = endpoint.Path
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
			options, err := connectionOptions(ctx, source, credentials)
			if err != nil {
				return "", err
			}
			defer options.close()
			if _, err := git.PlainCloneContext(ctx, filepath.Join(staging, ".git"), false, &git.CloneOptions{
				URL: source, NoCheckout: noCheckout, Auth: options.auth, CABundle: options.caBundle,
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
	remote, err := normalizationRemote(repo, preferOrigin)
	if err != nil {
		return "", err
	}
	remote = resolveFileRemote(repo, remote, layout.workingDirectory)
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
		options, err := remoteConnectionOptions(ctx, remote, credentials)
		if err != nil {
			return "", err
		}
		defer options.close()
		if err := remote.FetchContext(ctx, &git.FetchOptions{RefSpecs: specs, Auth: options.auth, CABundle: options.caBundle}); err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
			return "", transportFailure("fetch", err)
		}
	}
	return cfg.Name, nil
}

func normalizationRemote(repo *git.Repository, preferOrigin bool) (*git.Remote, error) {
	remotes, err := repo.Remotes()
	if err != nil {
		return nil, err
	}
	if len(remotes) == 1 {
		return remotes[0], nil
	}
	if preferOrigin && len(remotes) > 0 {
		for _, remote := range remotes {
			if strings.EqualFold(remote.Config().Name, "origin") {
				return remote, nil
			}
		}
		cfg, err := repo.Config()
		if err != nil {
			return nil, err
		}
		// The managed reader uses Git config file order when origin is absent.
		for _, section := range cfg.Raw.Section("remote").Subsections {
			if remote := cfg.Remotes[section.Name]; remote != nil {
				return git.NewRemote(repo.Storer, remote), nil
			}
		}
	}
	return nil, fmt.Errorf("%d remote(s) have been detected. When being run on a build server, the Git repository is expected to bear one (and no more than one) remote.", len(remotes))
}

// Git resolves local remote paths from the checkout, even when invoked from
// another directory. Leave the stored URL as the caller configured it.
func resolveFileRemote(repo *git.Repository, remote *git.Remote, directory string) *git.Remote {
	cfg := remote.Config()
	if len(cfg.URLs) == 0 {
		return remote
	}
	url := cfg.URLs[0]
	endpoint, err := transport.NewEndpoint(url)
	if err != nil || endpoint.Protocol != "file" || filepath.IsAbs(url) || strings.Contains(url, "://") {
		return remote
	}
	resolved := *cfg
	resolved.URLs = append([]string{filepath.Join(directory, url)}, cfg.URLs[1:]...)
	return git.NewRemote(repo.Storer, &resolved)
}

func remoteConnectionOptions(ctx context.Context, remote *git.Remote, credentials Credentials) (connection, error) {
	if urls := remote.Config().URLs; len(urls) > 0 {
		return connectionOptions(ctx, urls[0], credentials)
	}
	return connection{}, nil
}

func advertisedTips(ctx context.Context, remote *git.Remote, head plumbing.Hash, credentials Credentials) ([]string, error) {
	options, err := remoteConnectionOptions(ctx, remote, credentials)
	if err != nil {
		return nil, err
	}
	defer options.close()
	refs, err := remote.ListContext(ctx, &git.ListOptions{Auth: options.auth, CABundle: options.caBundle, PeelingOption: git.AppendPeeled})
	if err != nil {
		return nil, transportFailure("list remote references for", err)
	}
	tips := map[string]plumbing.Hash{}
	peeled := map[string]plumbing.Hash{}
	for _, ref := range refs {
		name := ref.Name().String()
		if !strings.HasPrefix(name, "refs/") || ref.Type() != plumbing.HashReference {
			continue
		}
		if base, ok := strings.CutSuffix(name, "^{}"); ok {
			peeled[base] = ref.Hash()
		} else {
			tips[name] = ref.Hash()
		}
	}
	// The in-process file transport does not advertise peeled tags. Resolve
	// them from its object store so that both transports use the commit tip.
	var local storer.Storer
	if ep, err := transport.NewEndpoint(remote.Config().URLs[0]); err == nil && ep.Protocol == "file" {
		local, err = (localRepositoryLoader{}).Load(ep)
		if err != nil {
			return nil, err
		}
	}
	var matching []string
	for name, sha := range tips {
		if target, ok := peeled[name]; ok {
			sha = target
		} else if local != nil && strings.HasPrefix(name, "refs/tags/") {
			sha = peelLocalTag(local, sha)
		}
		if sha == head {
			matching = append(matching, name)
		}
	}
	slices.Sort(matching)
	return matching, nil
}

func peelLocalTag(store storer.EncodedObjectStorer, hash plumbing.Hash) plumbing.Hash {
	seen := map[plumbing.Hash]bool{}
	for !seen[hash] {
		seen[hash] = true
		tag, err := object.GetTag(store, hash)
		if err != nil {
			// A lightweight tag already names its target object.
			return hash
		}
		hash = tag.Target
	}
	return plumbing.ZeroHash
}

func transportFailure(operation string, err error) error {
	var keyError *knownhosts.KeyError
	switch {
	case errors.As(err, &keyError), strings.Contains(err.Error(), "cannot create known hosts callback"):
		return errors.New("SSH host key verification failed")
	case strings.Contains(err.Error(), "ssh: unable to authenticate"):
		return errors.New("SSH authentication failed")
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
