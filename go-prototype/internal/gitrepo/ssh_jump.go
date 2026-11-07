package gitrepo

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/kevinburke/ssh_config"
	"golang.org/x/crypto/ssh"
	"golang.org/x/net/proxy"
)

// go-git accepts a ContextDialer through ProxyOptions. The private scheme
// connects it to the jump chain owned by this one repository operation.
const sshJumpScheme = "gitversion-ssh-jump"

var sshRoutes sync.Map
var sshRouteID atomic.Uint64

func init() {
	proxy.RegisterDialerType(sshJumpScheme, func(u *url.URL, _ proxy.Dialer) (proxy.Dialer, error) {
		if route, ok := sshRoutes.Load(u.Host); ok {
			return route.(sshJumpDialer), nil
		}
		return nil, fmt.Errorf("SSH jump route is unavailable")
	})
}

type sshJumpDialer struct {
	client      *ssh.Client
	destination string
	ctx         context.Context
}

func (d sshJumpDialer) Dial(network, address string) (net.Conn, error) {
	return d.DialContext(d.ctx, network, address)
}

func (d sshJumpDialer) DialContext(_ context.Context, network, _ string) (net.Conn, error) {
	conn, err := d.client.DialContext(d.ctx, network, d.destination)
	if err != nil {
		return nil, err
	}
	return sshForwardConn{Conn: conn}, nil
}

// A forwarded channel may already be closed by the peer after upload-pack
// exits. Present the same close behavior as a TCP connection to go-git.
type sshForwardConn struct{ net.Conn }

func (c sshForwardConn) Close() error {
	err := c.Conn.Close()
	if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
		return nil
	}
	return err
}

func openSSHJumpRoute(ctx context.Context, cfg *ssh_config.Config, home string, socket net.Conn, jumps, destination string) (transport.ProxyOptions, func(), error) {
	var hops []*ssh.Client
	closeHops := func() {
		for i := len(hops) - 1; i >= 0; i-- {
			_ = hops[i].Close()
		}
	}
	dial := (&net.Dialer{}).DialContext
	for _, jump := range strings.Split(jumps, ",") {
		endpoint, address, err := sshJumpEndpoint(cfg, strings.TrimSpace(jump))
		if err != nil {
			closeHops()
			return transport.ProxyOptions{}, nil, err
		}
		auth, err := newSSHAuth(endpoint, cfg, home, socket, os.Getenv("GITVERSION_SSH_JUMP_PASSWORD"), "")
		if err != nil {
			closeHops()
			return transport.ProxyOptions{}, nil, err
		}
		client, err := connectSSHJump(ctx, dial, address, auth)
		if err != nil {
			closeHops()
			return transport.ProxyOptions{}, nil, fmt.Errorf("SSH jump host: %w", transportFailure("connect to", err))
		}
		hops = append(hops, client)
		dial = client.DialContext
	}
	id := strconv.FormatUint(sshRouteID.Add(1), 10)
	sshRoutes.Store(id, sshJumpDialer{client: hops[len(hops)-1], destination: destination, ctx: ctx})
	var once sync.Once
	release := func() { once.Do(closeHops) }
	stop := context.AfterFunc(ctx, release)
	return transport.ProxyOptions{URL: sshJumpScheme + "://" + id}, func() {
		sshRoutes.Delete(id)
		stop()
		release()
	}, nil
}

func sshJumpEndpoint(cfg *ssh_config.Config, value string) (*transport.Endpoint, string, error) {
	if strings.Contains(value, "://") && !strings.HasPrefix(value, "ssh://") {
		return nil, "", fmt.Errorf("ProxyJump requires an SSH host")
	}
	if !strings.HasPrefix(value, "ssh://") {
		value = "ssh://" + value
	}
	u, err := url.Parse(value)
	if err != nil || u.Hostname() == "" || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return nil, "", fmt.Errorf("invalid SSH ProxyJump host")
	}
	if _, ok := u.User.Password(); ok {
		return nil, "", fmt.Errorf("SSH jump passwords belong in GITVERSION_SSH_JUMP_PASSWORD")
	}
	alias := u.Hostname()
	port, err := strconv.Atoi(u.Port())
	if u.Port() != "" && err != nil {
		return nil, "", fmt.Errorf("invalid SSH ProxyJump port")
	}
	address, err := sshConfiguredAddress(cfg, alias, port)
	if err != nil {
		return nil, "", err
	}
	return &transport.Endpoint{Host: alias, User: u.User.Username()}, address, nil
}

func sshConfiguredAddress(cfg *ssh_config.Config, alias string, port int) (string, error) {
	host, err := cfg.Get(alias, "HostName")
	if err != nil {
		return "", err
	}
	if host == "" {
		host = alias
	}
	if port == 0 {
		configured, err := cfg.Get(alias, "Port")
		if err != nil {
			return "", err
		}
		if configured == "" {
			configured = "22"
		}
		port, err = strconv.Atoi(configured)
		if err != nil {
			return "", fmt.Errorf("invalid SSH port")
		}
	}
	if port < 1 || port > 65535 {
		return "", fmt.Errorf("invalid SSH port")
	}
	return net.JoinHostPort(strings.Trim(host, "[]"), strconv.Itoa(port)), nil
}

func connectSSHJump(ctx context.Context, dial func(context.Context, string, string) (net.Conn, error), address string, auth sshPasswordAuth) (*ssh.Client, error) {
	cfg, err := auth.ClientConfig()
	if err != nil {
		return nil, err
	}
	raw, err := dial(ctx, "tcp", address)
	if err != nil {
		return nil, err
	}
	stop := context.AfterFunc(ctx, func() { _ = raw.Close() })
	conn, channels, requests, err := ssh.NewClientConn(raw, address, cfg)
	stop()
	if err != nil {
		_ = raw.Close()
		return nil, err
	}
	return ssh.NewClient(conn, channels, requests), nil
}
