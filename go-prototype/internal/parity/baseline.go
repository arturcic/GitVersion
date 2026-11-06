package parity

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"time"
)

type Spec struct {
	Schema        int               `json:"schema"`
	Revision      string            `json:"revision"`
	Project       string            `json:"project"`
	SDK           string            `json:"sdk"`
	Framework     string            `json:"framework"`
	Runtime       string            `json:"runtime"`
	Configuration string            `json:"configuration"`
	Environment   map[string]string `json:"environment"`
	Arguments     []string          `json:"arguments"`
}
type Baseline struct {
	Schema              int               `json:"schema"`
	SpecSHA256          string            `json:"spec_sha256"`
	SourceArchiveSHA256 string            `json:"source_archive_sha256"`
	Spec                Spec              `json:"spec"`
	Files               map[string]string `json:"files"`
	HostSHA256          string            `json:"host_sha256"`
	SDK                 string            `json:"sdk"`
	Runtimes            string            `json:"runtimes"`
	RuntimeFiles        map[string]string `json:"runtime_files"`
	Platform            string            `json:"platform"`
	GoVersion           string            `json:"go_version"`
	Help                string            `json:"help"`
	CLISchema           string            `json:"cli_schema"`
	Version             string            `json:"version"`
}

func Hash(data []byte) string { h := sha256.Sum256(data); return hex.EncodeToString(h[:]) }
func FileHash(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return Hash(data), nil
}
func ReadSpec(path string) (Spec, error) {
	var s Spec
	if err := ReadJSON(path, &s); err != nil {
		return s, err
	}
	if s.Schema != 1 || len(s.Revision) != 40 || s.Environment["GITVERSION_GIT_BACKEND"] != "managed" || s.Environment["GITVERSION_ARGUMENT_PARSER_VERSION"] != "v7" || s.Environment["GITVERSION_CONFIGURATION_VERSION"] != "v7" {
		return s, fmt.Errorf("invalid baseline identity or selectors")
	}
	if _, err := hex.DecodeString(s.Revision); err != nil {
		return s, fmt.Errorf("invalid revision")
	}
	return s, nil
}
func ReadJSON(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}
func WriteJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0644)
}

func FileSet(dir string) (map[string]string, error) {
	out := map[string]string{}
	err := filepath.WalkDir(dir, func(path string, e os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if e.IsDir() {
			return nil
		}
		info, err := e.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("nonregular baseline file %s", path)
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		h, err := FileHash(path)
		if err != nil {
			return err
		}
		out[filepath.ToSlash(rel)] = h
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("empty baseline")
	}
	return out, nil
}

func Inspect(ctx context.Context, dotnet, dir string, s Spec, args ...string) (string, error) {
	home, err := os.MkdirTemp("", "gitversion-oracle-home-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(home)
	runCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	c := exec.CommandContext(runCtx, dotnet, append([]string{"--fx-version", s.Runtime, filepath.Join(dir, "gitversion.dll")}, args...)...)
	c.Env = Environment(home, s.Environment)
	b, err := c.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("inspect %v: %w: %s", args, err, b)
	}
	return string(b), nil
}

func Pin(ctx context.Context, specPath, dotnet, dir, archive, goVersion string) (Baseline, error) {
	var b Baseline
	s, err := ReadSpec(specPath)
	if err != nil {
		return b, err
	}
	b.Schema = 1
	b.Spec = s
	b.Platform = Platform()
	b.GoVersion = goVersion
	if !filepath.IsAbs(dotnet) || !filepath.IsAbs(dir) {
		return b, fmt.Errorf("baseline and dotnet paths must be absolute")
	}
	if b.SpecSHA256, err = FileHash(specPath); err != nil {
		return b, err
	}
	if b.SourceArchiveSHA256, err = FileHash(archive); err != nil {
		return b, err
	}
	if b.Files, err = FileSet(dir); err != nil {
		return b, err
	}
	if _, ok := b.Files["gitversion.dll"]; !ok {
		return b, fmt.Errorf("missing gitversion.dll")
	}
	if b.HostSHA256, err = FileHash(dotnet); err != nil {
		return b, err
	}
	c := exec.CommandContext(ctx, dotnet, "--version")
	raw, err := c.Output()
	if err != nil {
		return b, err
	}
	b.SDK = strings.TrimSpace(string(raw))
	if b.SDK != s.SDK {
		return b, fmt.Errorf("SDK %s differs from pin %s", b.SDK, s.SDK)
	}
	raw, err = exec.CommandContext(ctx, dotnet, "--list-runtimes").Output()
	if err != nil {
		return b, err
	}
	b.Runtimes = string(raw)
	runtimeDir, err := RuntimeDirectory(b.Runtimes, s.Runtime)
	if err != nil {
		return b, err
	}
	if b.RuntimeFiles, err = FileSet(runtimeDir); err != nil {
		return b, err
	}
	if b.Help, err = Inspect(ctx, dotnet, dir, s, "--help"); err != nil {
		return b, err
	}
	for _, flag := range []string{"--no-cache", "--no-fetch", "--no-normalize", "--telemetry-opt-out", "--config", "--output"} {
		if !strings.Contains(b.Help, flag) {
			return b, fmt.Errorf("reference help missing %s", flag)
		}
	}
	if b.CLISchema, err = Inspect(ctx, dotnet, dir, s, "--cli-schema"); err != nil {
		return b, err
	}
	var schema any
	if err = json.Unmarshal([]byte(b.CLISchema), &schema); err != nil {
		return b, fmt.Errorf("invalid CLI schema: %w", err)
	}
	if b.Version, err = Inspect(ctx, dotnet, dir, s, "--version"); err != nil {
		return b, err
	}
	return b, nil
}

func VerifyBaseline(b Baseline, specPath, dotnet, dir string) error {
	if b.Schema != 1 {
		return fmt.Errorf("unsupported lock schema")
	}
	spec, err := ReadSpec(specPath)
	if err != nil {
		return err
	}
	hash, err := FileHash(specPath)
	if err != nil {
		return err
	}
	if hash != b.SpecSHA256 || !reflect.DeepEqual(spec, b.Spec) {
		return fmt.Errorf("baseline spec changed")
	}
	files, err := FileSet(dir)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(files, b.Files) {
		return fmt.Errorf("baseline files changed")
	}
	host, err := FileHash(dotnet)
	if err != nil {
		return err
	}
	if host != b.HostSHA256 {
		return fmt.Errorf("dotnet host changed")
	}
	raw, err := exec.Command(dotnet, "--list-runtimes").Output()
	if err != nil {
		return err
	}
	runtimeDir, err := RuntimeDirectory(string(raw), spec.Runtime)
	if err != nil {
		return err
	}
	runtimeFiles, err := FileSet(runtimeDir)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(runtimeFiles, b.RuntimeFiles) {
		return fmt.Errorf("pinned runtime files changed")
	}
	if Platform() != b.Platform {
		return fmt.Errorf("baseline platform changed; rebuild and repin")
	}
	return nil
}

// RuntimeDirectory selects the exact runtime version while preserving spaces in its installation path.
func RuntimeDirectory(inventory, version string) (string, error) {
	for _, line := range strings.Split(inventory, "\n") {
		prefix := "Microsoft.NETCore.App " + version + " ["
		if strings.HasPrefix(line, prefix) && strings.HasSuffix(strings.TrimSpace(line), "]") {
			base := strings.TrimSuffix(strings.TrimSpace(strings.TrimPrefix(line, prefix)), "]")
			return filepath.Join(base, version), nil
		}
	}
	return "", fmt.Errorf("pinned runtime %s is not installed", version)
}
