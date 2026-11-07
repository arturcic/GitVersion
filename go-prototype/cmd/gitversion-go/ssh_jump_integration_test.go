//go:build platform && !windows

package main

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	gitconfig "github.com/go-git/go-git/v5/config"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"

	"github.com/GitTools/GitVersion/go-prototype/internal/fixtures"
	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
)

const sshFixturePassword = "fixture-destination-secret"
const sshFixtureJumpPassword = "fixture-bastion-secret"
const sshFixtureWrongPassword = "fixture-incorrect-secret"

func fixturePasswordServer(user, password string) *ssh.ServerConfig {
	return &ssh.ServerConfig{PasswordCallback: func(conn ssh.ConnMetadata, supplied []byte) (*ssh.Permissions, error) {
		if conn.User() != user || string(supplied) != password {
			return nil, errors.New("fixture rejected password")
		}
		return nil, nil
	}}
}

func fixtureJumpServer(t *testing.T, target string, host ssh.Signer) (string, *atomic.Int32) {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	config := fixturePasswordServer("jump", sshFixtureJumpPassword)
	config.AddHostKey(host)
	var forwards atomic.Int32
	sshFixtureListener(t, listener, func(raw net.Conn) {
		conn, channels, requests, err := ssh.NewServerConn(raw, config)
		if err != nil {
			return
		}
		defer conn.Close()
		go ssh.DiscardRequests(requests)
		for channel := range channels {
			var endpoint struct {
				Host       string
				Port       uint32
				OriginHost string
				OriginPort uint32
			}
			if channel.ChannelType() != "direct-tcpip" || ssh.Unmarshal(channel.ExtraData(), &endpoint) != nil ||
				net.JoinHostPort(endpoint.Host, fmt.Sprint(endpoint.Port)) != target {
				_ = channel.Reject(ssh.Prohibited, "fixture accepts only its next loopback host")
				continue
			}
			next, err := net.DialTimeout("tcp", target, 5*time.Second)
			if err != nil {
				_ = channel.Reject(ssh.ConnectionFailed, "fixture next host unavailable")
				return
			}
			defer next.Close()
			stream, requests, err := channel.Accept()
			if err != nil {
				return
			}
			defer stream.Close()
			go ssh.DiscardRequests(requests)
			forwards.Add(1)
			copied := make(chan struct{})
			go func() {
				_, _ = io.Copy(next, stream)
				_ = next.(*net.TCPConn).CloseWrite()
				close(copied)
			}()
			_, _ = io.Copy(stream, next)
			_ = stream.Close()
			<-copied
			return
		}
	})
	return listener.Addr().String(), &forwards
}

func fixturePasswordSSHCommand(t *testing.T, env map[string]string, path, config string, hops []string, destination string) string {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if sshPath, err := exec.LookPath("ssh"); err == nil {
		ask := filepath.Join(path, "askpass")
		if err := os.WriteFile(ask, []byte("#!/bin/sh\nexec "+shellSSHArgument(self)+" --fixture-askpass \"$@\"\n"), 0700); err != nil {
			t.Fatal(err)
		}
		env["SSH_ASKPASS"] = ask
		env["SSH_ASKPASS_REQUIRE"] = "force"
		env["DISPLAY"] = "fixture"
		return shellSSHArgument(sshPath) + " -F " + shellSSHArgument(config)
	}
	env["GITVERSION_FIXTURE_DESTINATION"] = destination
	env["GITVERSION_FIXTURE_JUMPS"] = strings.Join(hops, ",")
	return shellSSHArgument(self) + " --fixture-ssh"
}

func TestPlatformSSHPasswordAndJumpParity(t *testing.T) {
	h := newRemoteCIHarness(t, "ssh-jump-")
	cases := []struct {
		name, mode, reject string
		hops               int
		cliPassword        bool
	}{
		{name: "password-clone", mode: "clone"},
		{name: "cli-password-clone", mode: "clone", cliPassword: true},
		{name: "wrong-password", mode: "clone", reject: "destination-password"},
		{name: "jump-clone", mode: "clone", hops: 1},
		{name: "two-jump-fetch", mode: "fetch", hops: 2},
		{name: "jump-detached-pr", mode: "pr", hops: 1},
		{name: "wrong-jump-password", mode: "clone", hops: 1, reject: "jump-password"},
		{name: "changed-jump-host-key", mode: "clone", hops: 1, reject: "jump-host"},
		{name: "changed-destination-host-key", mode: "clone", hops: 1, reject: "destination-host"},
	}
	var compared, matched int
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			source := filepath.Join(dir, "upstream.git")
			prepareExecutorCacheRepo(t, source)
			remote := remoteCIRepo(t, source)
			before := remoteCIHead(t, remote)
			seed := filepath.Join(dir, "seed")
			if err := fixtures.Copy(source, seed); err != nil {
				t.Fatal(err)
			}
			advanced := remoteCICommit(t, remote, 1)
			want, branch := advanced, "main"
			switch tc.mode {
			case "fetch":
				want = before
			case "pr":
				const pr = "refs/pull/47/merge"
				setRemoteCIRef(t, remote, pr, advanced)
				setRemoteCIRef(t, remote, "refs/heads/main", before)
				h.fixtureGit(t, seed, "fetch", "--quiet", source, pr)
				h.fixtureGit(t, seed, "checkout", "--quiet", "--detach", advanced.String())
				branch = "pull/47/merge"
			}
			_, destinationKey := sshFixtureKey(t)
			destination, uploads := sshFixtureUploadServer(t, h.git, source, destinationKey, fixturePasswordServer("git", sshFixturePassword))
			trust := destinationKey
			if tc.reject == "destination-host" {
				_, trust = sshFixtureKey(t)
			}
			lines := []string{knownhosts.Line([]string{destination}, trust.PublicKey())}
			addresses := make([]string, tc.hops)
			counts := make([]*atomic.Int32, tc.hops)
			aliases := make([]string, tc.hops)
			next := destination
			var hosts strings.Builder
			for i := tc.hops - 1; i >= 0; i-- {
				_, jumpKey := sshFixtureKey(t)
				addresses[i], counts[i] = fixtureJumpServer(t, next, jumpKey)
				trust = jumpKey
				if tc.reject == "jump-host" {
					_, trust = sshFixtureKey(t)
				}
				lines = append(lines, knownhosts.Line([]string{addresses[i]}, trust.PublicKey()))
				next = addresses[i]
				aliases[i] = fmt.Sprintf("fixture-jump-%d", i+1)
				host, port, err := net.SplitHostPort(next)
				if err != nil {
					t.Fatal(err)
				}
				fmt.Fprintf(&hosts, "Host %s\n HostName %s\n Port %s\n User jump\n", aliases[i], host, port)
			}
			authority := destination
			if tc.hops > 0 {
				host, port, err := net.SplitHostPort(destination)
				if err != nil {
					t.Fatal(err)
				}
				authority = "fixture-destination.invalid"
				fmt.Fprintf(&hosts, "Host %s\n HostName %s\n Port %s\n ProxyJump %s\n", authority, host, port, strings.Join(aliases, ","))
			}
			known := filepath.Join(dir, "known_hosts")
			sshFixtureFile(t, known, []byte(strings.Join(lines, "\n")+"\n"))
			fmt.Fprintf(&hosts, "Host *\n StrictHostKeyChecking yes\n UserKnownHostsFile %s\n GlobalKnownHostsFile /dev/null\n PreferredAuthentications password\n NumberOfPasswordPrompts 1\n", known)
			config := hosts.String()
			var outputs [2][]byte
			for i, implementation := range []string{"csharp", "go"} {
				checkout := filepath.Join(dir, implementation)
				home := filepath.Join(dir, implementation+"-home")
				cfgPath := filepath.Join(home, ".ssh", "config")
				sshFixtureFile(t, cfgPath, []byte(config))
				env := map[string]string{"HOME": home, "SSH_KNOWN_HOSTS": known, "GIT_SSH_VARIANT": "ssh", "ContinuaCI.Version": "true",
					"GITVERSION_SSH_PASSWORD": sshFixturePassword, "GITVERSION_SSH_JUMP_PASSWORD": sshFixtureJumpPassword}
				if tc.reject == "destination-password" {
					env["GITVERSION_SSH_PASSWORD"] = sshFixtureWrongPassword
				}
				if tc.reject == "jump-password" {
					env["GITVERSION_SSH_JUMP_PASSWORD"] = sshFixtureWrongPassword
				}
				if implementation == "csharp" {
					env["GIT_SSH_COMMAND"] = fixturePasswordSSHCommand(t, env, home, cfgPath, addresses, destination)
				} else {
					env["GIT_SSH_COMMAND"] = filepath.Join(home, "nonexistent-ssh")
				}
				url := "ssh://git@" + authority + source
				var args []string
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
					if tc.mode == "pr" {
						args = append(args, "--no-fetch")
					} else {
						env["GIT_BRANCH"] = "main"
					}
				}
				if tc.cliPassword && implementation == "go" {
					env["GITVERSION_SSH_PASSWORD"] = sshFixtureWrongPassword
					args = append(args, "--password", sshFixturePassword)
				}
				csharpFailure, goFailure := "", ""
				switch tc.reject {
				case "destination-password", "jump-password":
					csharpFailure, goFailure = "Permission denied", "SSH authentication failed"
				case "jump-host", "destination-host":
					csharpFailure, goFailure = "Host key verification failed", "SSH host key verification failed"
				}
				u := uploads.Load()
				start := make([]int32, len(counts))
				for i, count := range counts {
					start[i] = count.Load()
				}
				outputs[i] = h.runWithRejection(t, implementation, checkout, env, args, csharpFailure, goFailure)
				if (uploads.Load() > u) != (tc.reject == "") {
					t.Fatalf("%s: unexpected upload-pack activity", implementation)
				}
				for i, count := range counts {
					expected := tc.reject == "" || tc.reject == "destination-host"
					if (count.Load() > start[i]) != expected {
						t.Fatalf("%s: unexpected forwarding at hop %d", implementation, i+1)
					}
				}
				area := filepath.Join(h.report, tc.name, implementation)
				paths := []string{filepath.Join(area, "stdout"), filepath.Join(area, "stderr"), filepath.Join(area, "gitversion.log")}
				if tc.reject == "" {
					repoPath := checkout
					if tc.mode == "clone" {
						repoPath = filepath.Join(checkout, "clones", "upstream", ".git")
					}
					paths = append(paths, filepath.Join(repoPath, ".git", "config"))
				}
				for _, path := range paths {
					data, err := os.ReadFile(path)
					if err != nil && filepath.Base(path) == "gitversion.log" && errors.Is(err, os.ErrNotExist) {
						continue
					}
					if err != nil {
						t.Fatal(err)
					}
					for _, secret := range []string{sshFixturePassword, sshFixtureJumpPassword, sshFixtureWrongPassword} {
						if strings.Contains(string(data), secret) {
							t.Fatalf("%s: fixture password leaked to %s", implementation, filepath.Base(path))
						}
					}
				}
				if tc.mode == "fetch" {
					repo := remoteCIRepo(t, checkout)
					ref, err := repo.Reference("refs/remotes/origin/main", true)
					if err != nil || ref.Hash() != advanced || remoteCIHead(t, repo) != before {
						t.Fatalf("%s: fetch state mismatch (%v)", implementation, err)
					}
				}
			}
			if tc.reject == "" {
				verifyRemoteCIResult(t, outputs[0], outputs[1], want, branch)
				matched++
			}
			compared++
		})
	}
	if !t.Failed() && compared == len(cases) {
		hash, err := parity.FileHash(h.candidate)
		if err != nil {
			t.Fatal(err)
		}
		client := "OpenSSH"
		if _, err := exec.LookPath("ssh"); err != nil {
			client = "fixture SSH client"
		}
		if err := parity.WriteJSON(filepath.Join(h.report, "report.json"), map[string]any{
			"comparisons": compared, "json_comparisons": matched, "rejection_comparisons": compared - matched,
			"baseline_revision": h.baseline.Spec.Revision, "baseline_files": h.baseline.Files,
			"candidate_sha256": hash, "spec_sha256": h.baseline.SpecSHA256, "csharp_backend": "managed", "oracle_ssh_client": client,
		}); err != nil {
			t.Fatal(err)
		}
	}
}
