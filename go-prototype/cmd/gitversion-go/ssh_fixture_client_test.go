//go:build platform && !windows

package main

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

// The Ubuntu parity image has no OpenSSH executable. This test-only client
// implements the part of Git's external SSH interface needed by the fixture.
func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == "--fixture-ssh" {
		if err := runFixtureSSHClient(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func runFixtureSSHClient(args []string) error {
	port := "22"
	for len(args) > 0 && strings.HasPrefix(args[0], "-") {
		if len(args) < 2 {
			return fmt.Errorf("missing fixture SSH option value")
		}
		switch args[0] {
		case "-o":
		case "-p":
			port = args[1]
		default:
			return fmt.Errorf("unexpected fixture SSH option %q", args[0])
		}
		args = args[2:]
	}
	if len(args) != 2 {
		return fmt.Errorf("fixture SSH expected destination and command")
	}
	user, host, ok := strings.Cut(args[0], "@")
	if !ok || host != "127.0.0.1" {
		return fmt.Errorf("fixture SSH accepts only loopback destinations")
	}
	trust, err := knownhosts.New(os.Getenv("SSH_KNOWN_HOSTS"))
	if err != nil {
		return err
	}
	var signers []ssh.Signer
	if socket := os.Getenv("SSH_AUTH_SOCK"); socket != "" {
		conn, err := net.DialTimeout("unix", socket, 5*time.Second)
		if err != nil {
			return err
		}
		defer conn.Close()
		signers, err = agent.NewClient(conn).Signers()
		if err != nil {
			return err
		}
	}
	if path := os.Getenv("GITVERSION_FIXTURE_IDENTITY"); path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		key, err := ssh.ParsePrivateKey(data)
		if err == nil {
			signers = []ssh.Signer{key}
		} else {
			var locked *ssh.PassphraseMissingError
			if !errors.As(err, &locked) || locked.PublicKey == nil {
				return err
			}
			var matches []ssh.Signer
			for _, signer := range signers {
				if bytes.Equal(signer.PublicKey().Marshal(), locked.PublicKey.Marshal()) {
					matches = append(matches, signer)
				}
			}
			signers = matches
		}
	}

	client, err := ssh.Dial("tcp", net.JoinHostPort(host, port), &ssh.ClientConfig{
		User: user, Auth: []ssh.AuthMethod{ssh.PublicKeys(signers...)}, HostKeyCallback: trust, Timeout: 5 * time.Second,
	})
	if err != nil {
		var hostError *knownhosts.KeyError
		switch {
		case errors.As(err, &hostError):
			return fmt.Errorf("Host key verification failed")
		case strings.Contains(err.Error(), "unable to authenticate"):
			return fmt.Errorf("Permission denied (publickey)")
		default:
			return err
		}
	}
	defer client.Close()
	session, err := client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()
	session.Stdin, session.Stdout, session.Stderr = os.Stdin, os.Stdout, os.Stderr
	return session.Run(args[1])
}
