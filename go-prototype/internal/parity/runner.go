package parity

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/GitTools/GitVersion/go-prototype/internal/fixtures"
)

type Command struct {
	Executable string   `json:"executable"`
	Arguments  []string `json:"arguments"`
}
type Result struct {
	Command      Command  `json:"command"`
	Environment  []string `json:"environment"`
	ExitCode     int      `json:"exit_code"`
	DurationMS   int64    `json:"duration_ms"`
	DurationNS   int64    `json:"duration_ns"`
	PeakRSSBytes int64    `json:"peak_rss_bytes,omitempty"`
	Error        string   `json:"error,omitempty"`
	Before       string   `json:"before"`
	After        string   `json:"after"`
	Stdout       []byte   `json:"-"`
	Stderr       []byte   `json:"-"`
}

const maxOutputBytes = 4 << 20

type limitedBuffer struct {
	buffer   bytes.Buffer
	exceeded bool
}

func (b *limitedBuffer) Bytes() []byte { return b.buffer.Bytes() }

func (b *limitedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	remaining := maxOutputBytes - b.buffer.Len()
	if len(p) > remaining {
		p = p[:remaining]
		b.exceeded = true
	}
	_, err := b.buffer.Write(p)
	return n, err
}

// Environment deliberately does not inherit CI selectors, credentials, Git config,
// branch context or user formatting settings. HOME is per-process scratch.
func Environment(home string, extra map[string]string) []string {
	env := map[string]string{}
	for _, key := range []string{"PATH", "SystemRoot", "WINDIR", "COMSPEC", "PATHEXT"} {
		if v := os.Getenv(key); v != "" {
			env[key] = v
		}
	}
	for k, v := range map[string]string{
		"HOME": home, "USERPROFILE": home, "XDG_CONFIG_HOME": home, "DOTNET_CLI_HOME": home,
		"TMPDIR": home, "TMP": home, "TEMP": home, "TZ": "UTC", "LANG": "C.UTF-8", "LC_ALL": "C.UTF-8",
		"GIT_CONFIG_NOSYSTEM": "1", "GIT_CONFIG_GLOBAL": filepath.Join(home, "gitconfig"), "GIT_TERMINAL_PROMPT": "0",
		"DOTNET_CLI_TELEMETRY_OPTOUT": "1", "DOTNET_SKIP_FIRST_TIME_EXPERIENCE": "1", "DOTNET_NOLOGO": "1",
		"DOTNET_SYSTEM_GLOBALIZATION_INVARIANT": "1", "GITVERSION_TELEMETRY_OPTOUT": "1",
	} {
		env[k] = v
	}
	for k, v := range extra {
		env[k] = v
	}
	keys := []string{}
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := []string{}
	for _, k := range keys {
		out = append(out, k+"="+env[k])
	}
	return out
}

func Resolve(c Command, repo string) (Command, error) {
	if !filepath.IsAbs(c.Executable) {
		return c, fmt.Errorf("executable must be absolute")
	}
	args := append([]string(nil), c.Arguments...)
	found := false
	for i, a := range args {
		if a == "{repo}" {
			args[i] = repo
			found = true
		} else if strings.Contains(a, "{repo}") {
			return c, fmt.Errorf("{repo} must be a complete argument")
		}
	}
	if !found {
		return c, fmt.Errorf("command requires a {repo} argument")
	}
	return Command{c.Executable, args}, nil
}

// Run records failures rather than hiding them behind successful JSON parsing.
// Each run gets its own repository copy and private HOME; it never invokes a shell.
func Run(ctx context.Context, c Command, source, workspace string, environment map[string]string, timeout time.Duration) (Result, error) {
	var result Result
	if timeout <= 0 {
		return result, fmt.Errorf("timeout must be positive")
	}
	if err := os.Mkdir(workspace, 0755); err != nil {
		return result, err
	}
	repo := filepath.Join(workspace, "repo")
	if err := fixtures.Copy(source, repo); err != nil {
		return result, err
	}
	resolved, err := Resolve(c, repo)
	if err != nil {
		return result, err
	}
	result.Command = resolved
	home := filepath.Join(workspace, "home")
	if err = os.Mkdir(home, 0700); err != nil {
		return result, err
	}
	result.Before, err = fixtures.State(repo)
	if err != nil {
		return result, err
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(runCtx, resolved.Executable, resolved.Arguments...)
	cmd.Dir = repo
	cmd.Env = Environment(home, environment)
	result.Environment = cmd.Env
	var stdout, stderr limitedBuffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	// Bound waits if a child inherits pipes. Candidates are trusted local commands.
	cmd.WaitDelay = time.Second
	start := time.Now()
	err = cmd.Run()
	duration := time.Since(start)
	result.DurationMS = duration.Milliseconds()
	result.DurationNS = duration.Nanoseconds()
	if cmd.ProcessState != nil {
		result.PeakRSSBytes = peakRSS(cmd.ProcessState)
	}
	result.ExitCode = 0
	if err != nil {
		result.ExitCode = -1
		if cmd.ProcessState != nil {
			result.ExitCode = cmd.ProcessState.ExitCode()
		}
		result.Error = err.Error()
	}
	if runCtx.Err() != nil {
		result.Error = runCtx.Err().Error()
	}
	if stdout.exceeded || stderr.exceeded {
		result.Error = strings.TrimSpace(result.Error + " output exceeded 4 MiB per stream")
	}
	result.Stdout = stdout.Bytes()
	result.Stderr = stderr.Bytes()
	result.After, err = fixtures.State(repo)
	if err != nil {
		result.Error = strings.TrimSpace(result.Error + " cannot inspect repository after execution: " + err.Error())
		return result, nil
	}
	if result.Before != result.After {
		result.Error = strings.TrimSpace(result.Error + " repository mutated")
	}
	return result, nil
}

func (r Result) Validate() error {
	if r.Error != "" || r.ExitCode != 0 {
		return fmt.Errorf("exit %d: %s", r.ExitCode, r.Error)
	}
	_, err := Object(r.Stdout)
	return err
}

func Platform() string { return runtime.GOOS + "/" + runtime.GOARCH }
