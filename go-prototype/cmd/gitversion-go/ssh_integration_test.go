//go:build platform && !windows

package main

import (
	"bytes"
	"crypto/ed25519"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	gitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"

	"github.com/GitTools/GitVersion/go-prototype/internal/fixtures"
	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
)

func sshFixtureKey(t *testing.T) (ed25519.PrivateKey, ssh.Signer) {
	t.Helper()
	_, key, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return key, signer
}

func sshFixtureFile(t *testing.T, path string, contents []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, contents, 0600); err != nil {
		t.Fatal(err)
	}
}

// Keep fixtures local and bound the lifetime of connections even on test failure.
func sshFixtureListener(t *testing.T, listener net.Listener, serve func(net.Conn)) {
	t.Helper()
	var clients sync.WaitGroup
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			clients.Go(func() {
				defer conn.Close()
				_ = conn.SetDeadline(time.Now().Add(20 * time.Second))
				serve(conn)
			})
		}
	}()
	t.Cleanup(func() {
		_ = listener.Close()
		<-finished
		clients.Wait()
	})
}

func sshFixtureAgent(t *testing.T, key ed25519.PrivateKey) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "gv-ssh-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	socket := filepath.Join(dir, "agent")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	keyring := agent.NewKeyring()
	if err := keyring.Add(agent.AddedKey{PrivateKey: key}); err != nil {
		t.Fatal(err)
	}
	sshFixtureListener(t, listener, func(conn net.Conn) { _ = agent.ServeAgent(keyring, conn) })
	return socket
}

func sshFixtureServer(t *testing.T, gitPath, repository string, host ssh.Signer, allowed ssh.PublicKey) (string, *atomic.Int32) {
	t.Helper()
	server := &ssh.ServerConfig{PublicKeyCallback: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
		if conn.User() != "git" || !bytes.Equal(key.Marshal(), allowed.Marshal()) {
			return nil, fmt.Errorf("fixture rejected client key")
		}
		return nil, nil
	}}
	return sshFixtureUploadServer(t, gitPath, repository, host, server)
}

func sshFixtureUploadServer(t *testing.T, gitPath, repository string, host ssh.Signer, server *ssh.ServerConfig) (string, *atomic.Int32) {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server.AddHostKey(host)
	var uploads atomic.Int32
	sshFixtureListener(t, listener, func(conn net.Conn) {
		session, channels, requests, err := ssh.NewServerConn(conn, server)
		if err != nil {
			return
		}
		defer session.Close()
		go ssh.DiscardRequests(requests)
		for request := range channels {
			if request.ChannelType() != "session" {
				_ = request.Reject(ssh.UnknownChannelType, "session channels only")
				continue
			}
			channel, requests, err := request.Accept()
			if err != nil {
				return
			}
			defer channel.Close()
			for req := range requests {
				var command struct{ Command string }
				if req.Type != "exec" || ssh.Unmarshal(req.Payload, &command) != nil ||
					command.Command != "git-upload-pack '"+repository+"'" {
					_ = req.Reply(false, nil)
					continue
				}
				_ = req.Reply(true, nil)
				uploads.Add(1)
				cmd := exec.CommandContext(t.Context(), gitPath, "upload-pack", repository)
				cmd.Stdout, cmd.Stderr = channel, channel.Stderr()
				input, err := cmd.StdinPipe()
				if err != nil {
					return
				}
				go func() {
					_, _ = io.Copy(input, channel)
					_ = input.Close()
				}()
				cmd.Env = parity.Environment(filepath.Dir(repository), nil)
				var status uint32
				if err := cmd.Run(); err != nil {
					status = 1
				}
				_, _ = channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Code uint32 }{status}))
				return
			}
		}
	})
	return listener.Addr().String(), &uploads
}

func sshFixtureClient(t *testing.T, env map[string]string, known, identity string) string {
	t.Helper()
	if sshPath, err := exec.LookPath("ssh"); err == nil {
		command := shellSSHArgument(sshPath) + " -F /dev/null -o BatchMode=yes -o StrictHostKeyChecking=yes -o GlobalKnownHostsFile=/dev/null -o " +
			shellSSHArgument("UserKnownHostsFile="+known)
		if identity != "" {
			command += " -o IdentitiesOnly=yes -i " + shellSSHArgument(identity)
		}
		return command
	}
	// The offline Ubuntu image has no OpenSSH client. Git still invokes an
	// independent SSH executable through its normal GIT_SSH_COMMAND interface.
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	env["GITVERSION_FIXTURE_IDENTITY"] = identity
	return shellSSHArgument(self) + " --fixture-ssh"
}

func shellSSHArgument(arg string) string {
	return "'" + strings.ReplaceAll(arg, "'", "'\\''") + "'"
}

func TestPlatformSSHParity(t *testing.T) {
	h := newRemoteCIHarness(t, "ssh-parity-")
	cases := []struct {
		name, mode, auth       string
		wrongHost, wrongClient bool
	}{
		{name: "agent-clone", mode: "clone", auth: "agent"},
		{name: "default-identity-clone", mode: "clone", auth: "default"},
		{name: "configured-identity-clone", mode: "clone", auth: "configured"},
		{name: "configured-encrypted-agent-identity", mode: "clone", auth: "encrypted"},
		{name: "agent-fetch", mode: "fetch", auth: "agent"},
		{name: "agent-detached-pr", mode: "pr", auth: "agent"},
		{name: "ci-no-fetch", mode: "no-fetch"},
		{name: "reject-wrong-client-key", mode: "clone", auth: "agent", wrongClient: true},
		{name: "reject-changed-host-key", mode: "clone", auth: "agent", wrongHost: true},
	}
	var completed, matched int
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "upstream.git")
			prepareExecutorCacheRepo(t, path)
			remote := remoteCIRepo(t, path)
			before := remoteCIHead(t, remote)
			seed := filepath.Join(dir, "seed")
			if err := fixtures.Copy(path, seed); err != nil {
				t.Fatal(err)
			}
			advanced := remoteCICommit(t, remote, 1)
			head, branch := before, "main"
			if tc.mode == "pr" {
				const pr = "refs/pull/42/merge"
				setRemoteCIRef(t, remote, pr, advanced)
				setRemoteCIRef(t, remote, "refs/heads/main", before)
				h.fixtureGit(t, seed, "fetch", "--quiet", path, pr)
				h.fixtureGit(t, seed, "checkout", "--quiet", "--detach", advanced.String())
				head, branch = advanced, "pull/42/merge"
			} else if tc.mode == "clone" {
				head = advanced
			}
			key, client := sshFixtureKey(t)
			_, host := sshFixtureKey(t)
			address, uploads := sshFixtureServer(t, h.git, path, host, client.PublicKey())
			hostForTrust := host
			if tc.wrongHost {
				_, hostForTrust = sshFixtureKey(t)
			}
			if tc.wrongClient {
				key, _ = sshFixtureKey(t)
			}
			known := filepath.Join(dir, "known_hosts")
			sshFixtureFile(t, known, []byte(knownhosts.Line([]string{address}, hostForTrust.PublicKey())+"\n"))
			socket := ""
			if tc.auth == "agent" || tc.auth == "encrypted" {
				socket = sshFixtureAgent(t, key)
			}
			var ref, result []byte
			for _, implementation := range []string{"csharp", "go"} {
				checkout := filepath.Join(dir, implementation)
				home := filepath.Join(dir, implementation+"-home")
				env := map[string]string{
					"HOME": home, "SSH_KNOWN_HOSTS": known, "SSH_AUTH_SOCK": socket,
					"GIT_SSH_VARIANT": "ssh", "ContinuaCI.Version": "true",
				}
				var identity string
				if tc.auth == "default" || tc.auth == "configured" || tc.auth == "encrypted" {
					identity = filepath.Join(home, ".ssh", "id_ed25519")
					if tc.auth == "configured" || tc.auth == "encrypted" {
						identity = filepath.Join(home, ".ssh", "custom-identity")
						sshFixtureFile(t, filepath.Join(home, ".ssh", "config"), []byte("Host 127.0.0.1\n  IdentityFile ~/.ssh/custom-identity\n  IdentitiesOnly yes\n"))
					}
					var block *pem.Block
					var err error
					if tc.auth == "encrypted" {
						block, err = ssh.MarshalPrivateKeyWithPassphrase(key, "", []byte("unused-fixture-passphrase"))
					} else {
						block, err = ssh.MarshalPrivateKey(key, "")
					}
					if err != nil {
						t.Fatal(err)
					}
					sshFixtureFile(t, identity, pem.EncodeToMemory(block))
				} else if err := os.Mkdir(home, 0700); err != nil {
					t.Fatal(err)
				}
				oracleKnownHosts := known
				if tc.auth == "default" {
					oracleKnownHosts = filepath.Join(home, ".ssh", "known_hosts")
					sshFixtureFile(t, oracleKnownHosts, []byte(knownhosts.Line([]string{address}, hostForTrust.PublicKey())+"\n"))
					delete(env, "SSH_KNOWN_HOSTS")
				}
				if implementation == "csharp" {
					env["SSH_KNOWN_HOSTS"] = oracleKnownHosts
					env["GIT_SSH_COMMAND"] = sshFixtureClient(t, env, oracleKnownHosts, identity)
				} else {
					env["GIT_SSH_COMMAND"] = filepath.Join(home, "nonexistent-ssh")
				}
				url := "ssh://git@" + address + path
				args := []string{}
				if tc.mode == "clone" {
					if err := os.Mkdir(checkout, 0700); err != nil {
						t.Fatal(err)
					}
					args = []string{"--url", url, "--dynamic-repo-location", filepath.Join(checkout, "clones"), "--branch", "main", "--no-fetch"}
				} else {
					if err := fixtures.Copy(seed, checkout); err != nil {
						t.Fatal(err)
					}
					repo := remoteCIRepo(t, checkout)
					if _, err := repo.CreateRemote(&gitconfig.RemoteConfig{Name: "origin", URLs: []string{url}}); err != nil {
						t.Fatal(err)
					}
					if tc.mode == "pr" || tc.mode == "no-fetch" {
						args = append(args, "--no-fetch")
					} else {
						env["GIT_BRANCH"] = "main"
					}
				}
				csharpFailure, goFailure := "", ""
				switch {
				case tc.wrongClient:
					csharpFailure, goFailure = "Permission denied", "SSH authentication failed"
				case tc.wrongHost:
					csharpFailure, goFailure = "Host key verification failed", "SSH host key verification failed"
				}
				initialUploads := uploads.Load()
				output := h.runWithRejection(t, implementation, checkout, env, args, csharpFailure, goFailure)
				wantUpload := tc.mode != "no-fetch" && goFailure == ""
				if (uploads.Load() > initialUploads) != wantUpload {
					t.Fatalf("%s: unexpected SSH upload-pack activity", implementation)
				}
				if tc.mode == "fetch" || tc.mode == "no-fetch" {
					repo := remoteCIRepo(t, checkout)
					tracking, err := repo.Reference("refs/remotes/origin/main", true)
					if tc.mode == "fetch" && (err != nil || tracking.Hash() != advanced) ||
						tc.mode == "no-fetch" && err != plumbing.ErrReferenceNotFound || remoteCIHead(t, repo) != before {
						t.Fatalf("%s: unexpected tracking ref %v (%v)", implementation, tracking, err)
					}
				}
				if implementation == "csharp" {
					ref = output
				} else {
					result = output
				}
			}
			if !tc.wrongClient && !tc.wrongHost {
				verifyRemoteCIResult(t, ref, result, head, branch)
				matched++
			}
			completed++
		})
	}
	if !t.Failed() && completed == len(cases) {
		hash, err := parity.FileHash(h.candidate)
		if err != nil {
			t.Fatal(err)
		}
		client := "OpenSSH"
		if _, err := exec.LookPath("ssh"); err != nil {
			client = "fixture SSH client"
		}
		if err := parity.WriteJSON(filepath.Join(h.report, "report.json"), map[string]any{
			"comparisons": completed, "json_comparisons": matched, "rejection_comparisons": completed - matched,
			"baseline_revision": h.baseline.Spec.Revision, "baseline_files": h.baseline.Files,
			"candidate_sha256": hash, "spec_sha256": h.baseline.SpecSHA256, "csharp_backend": "managed", "oracle_ssh_client": client,
		}); err != nil {
			t.Fatal(err)
		}
	}
}
