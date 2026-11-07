package gitrepo

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/transport"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/kevinburke/ssh_config"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

var errSSHPasswordInURL = errors.New("SSH passwords in remote URLs are unsupported. Use GITVERSION_SSH_PASSWORD.")

func sshConnectionOptions(ctx context.Context, endpoint *transport.Endpoint, credentials Credentials) (connection, error) {
	if endpoint.Password != "" {
		return connection{}, errSSHPasswordInURL
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return connection{}, fmt.Errorf("resolve SSH home directory: %w", err)
	}
	cfg, err := readSSHUserConfig(home)
	if err != nil {
		return connection{}, err
	}
	// Agent signers require the socket for the lifetime of the operation.
	var options connection
	if address := os.Getenv("SSH_AUTH_SOCK"); address != "" {
		options.socket, _ = (&net.Dialer{}).DialContext(ctx, "unix", address)
		if deadline, ok := ctx.Deadline(); ok && options.socket != nil {
			_ = options.socket.SetDeadline(deadline)
		}
	}
	password := credentials.Password
	if password == "" {
		password = os.Getenv("GITVERSION_SSH_PASSWORD")
	}
	auth, err := newSSHAuth(endpoint, cfg, home, options.socket, password, credentials.Username)
	if err != nil {
		options.close()
		return connection{}, err
	}
	options.auth = auth
	if cfg != nil {
		jumps, err := cfg.Get(endpoint.Host, "ProxyJump")
		if err == nil && jumps != "" && !strings.EqualFold(jumps, "none") {
			auth.hostKeyAddress, err = sshConfiguredAddress(cfg, strings.Trim(endpoint.Host, "[]"), endpoint.Port)
			if err == nil {
				options.proxy, options.closeRoute, err = openSSHJumpRoute(ctx, cfg, home, options.socket, jumps, auth.hostKeyAddress)
			}
			options.auth = auth
		}
		if err != nil {
			options.close()
			return connection{}, err
		}
	}
	return options, nil
}

type sshPasswordAuth struct {
	*gitssh.PublicKeysCallback
	password       string
	hostKeyAddress string
}

func (a sshPasswordAuth) ClientConfig() (*ssh.ClientConfig, error) {
	cfg, err := a.PublicKeysCallback.ClientConfig()
	if err != nil {
		return nil, err
	}
	if a.password != "" {
		cfg.Auth = append(cfg.Auth, ssh.Password(a.password))
	}
	if a.hostKeyAddress != "" {
		verify := cfg.HostKeyCallback
		cfg.HostKeyCallback = func(_ string, remote net.Addr, key ssh.PublicKey) error {
			return verify(a.hostKeyAddress, remote, key)
		}
	}
	return cfg, nil
}

func newSSHAuth(endpoint *transport.Endpoint, cfg *ssh_config.Config, home string, socket net.Conn, password, fallbackUser string) (sshPasswordAuth, error) {
	var err error
	username := endpoint.User
	if username == "" {
		username = fallbackUser
	}
	var identities []string
	var identitiesOnly string
	if cfg != nil {
		if username == "" {
			username, err = cfg.Get(endpoint.Host, "User")
			if err != nil {
				return sshPasswordAuth{}, fmt.Errorf("resolve SSH username: %w", err)
			}
		}
		identities, err = cfg.GetAll(endpoint.Host, "IdentityFile")
		if err != nil {
			return sshPasswordAuth{}, fmt.Errorf("resolve SSH identities: %w", err)
		}
		identitiesOnly, err = cfg.Get(endpoint.Host, "IdentitiesOnly")
		if err != nil {
			return sshPasswordAuth{}, fmt.Errorf("resolve SSH identity policy: %w", err)
		}
	}
	if username == "" {
		current, err := user.Current()
		if err != nil {
			return sshPasswordAuth{}, fmt.Errorf("resolve SSH username: %w", err)
		}
		username = current.Username
	}
	if len(identities) == 0 {
		identities = []string{"~/.ssh/id_ed25519", "~/.ssh/id_ecdsa", "~/.ssh/id_rsa"}
	}
	auth := &gitssh.PublicKeysCallback{User: username, Callback: func() ([]ssh.Signer, error) {
		var files []ssh.Signer
		selected := map[string]bool{}
		for _, path := range identities {
			if strings.EqualFold(path, "none") {
				continue
			}
			if relative, ok := strings.CutPrefix(path, "~/"); ok {
				path = filepath.Join(home, relative)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			key, err := ssh.ParsePrivateKey(data)
			if err == nil {
				files = append(files, key)
				selected[string(key.PublicKey().Marshal())] = true
				continue
			}
			// OpenSSH can select an agent key from its public key file or from the
			// public portion of an encrypted identity.
			var locked *ssh.PassphraseMissingError
			if errors.As(err, &locked) && locked.PublicKey != nil {
				selected[string(locked.PublicKey.Marshal())] = true
			} else if public, _, _, _, err := ssh.ParseAuthorizedKey(data); err == nil {
				selected[string(public.Marshal())] = true
			}
		}
		var signers []ssh.Signer
		if socket != nil {
			available, _ := agent.NewClient(socket).Signers()
			for _, key := range available {
				if !strings.EqualFold(identitiesOnly, "yes") || selected[string(key.PublicKey().Marshal())] {
					signers = append(signers, key)
				}
			}
		}
		return append(signers, files...), nil
	}}

	return sshPasswordAuth{PublicKeysCallback: auth, password: password}, nil
}

func readSSHUserConfig(home string) (*ssh_config.Config, error) {
	file, err := os.Open(filepath.Join(home, ".ssh", "config"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open SSH user configuration: %w", err)
	}
	defer file.Close()
	cfg, err := ssh_config.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("parse SSH user configuration: %w", err)
	}
	return cfg, nil
}
