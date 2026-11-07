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

func sshConnectionOptions(ctx context.Context, endpoint *transport.Endpoint) (connection, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return connection{}, fmt.Errorf("resolve SSH home directory: %w", err)
	}
	cfg, err := readSSHUserConfig(home)
	if err != nil {
		return connection{}, err
	}
	username := endpoint.User
	var identities []string
	var identitiesOnly string
	if cfg != nil {
		if username == "" {
			username, err = cfg.Get(endpoint.Host, "User")
			if err != nil {
				return connection{}, fmt.Errorf("resolve SSH username: %w", err)
			}
		}
		identities, err = cfg.GetAll(endpoint.Host, "IdentityFile")
		if err != nil {
			return connection{}, fmt.Errorf("resolve SSH identities: %w", err)
		}
		identitiesOnly, err = cfg.Get(endpoint.Host, "IdentitiesOnly")
		if err != nil {
			return connection{}, fmt.Errorf("resolve SSH identity policy: %w", err)
		}
	}
	if username == "" {
		current, err := user.Current()
		if err != nil {
			return connection{}, fmt.Errorf("resolve SSH username: %w", err)
		}
		username = current.Username
	}
	if len(identities) == 0 {
		identities = []string{"~/.ssh/id_ed25519", "~/.ssh/id_ecdsa", "~/.ssh/id_rsa"}
	}
	// An unavailable agent is equivalent to having no agent identities. Keep
	// the socket open through the operation because agent signers use it.
	var socket net.Conn
	if address := os.Getenv("SSH_AUTH_SOCK"); address != "" {
		socket, _ = (&net.Dialer{}).DialContext(ctx, "unix", address)
		if deadline, ok := ctx.Deadline(); ok && socket != nil {
			_ = socket.SetDeadline(deadline)
		}
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

	return connection{auth: auth, socket: socket}, nil
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
