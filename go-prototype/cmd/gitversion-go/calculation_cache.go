package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/GitTools/GitVersion/go-prototype/internal/gitrepo"
	"github.com/GitTools/GitVersion/go-prototype/internal/output"
)

func calculationCachePath(repo *gitrepo.Repository, version, commit string, configuration []byte) (string, error) {
	branch, head := repo.Head()
	refs, err := repo.References()
	if err != nil {
		return "", err
	}
	identity := struct {
		Branch, Head, RequestedCommit, ConfigurationVersion string
		Configuration                                       []byte
		References                                          []gitrepo.Reference
		ShallowCommits                                      []string
		UncommittedChanges                                  int
		ContextualHead                                      bool `json:",omitempty"`
	}{branch, head, commit, version, configuration, refs, repo.ShallowCommits(), repo.UncommittedChanges(), repo.ContextualHead()}
	input, err := json.Marshal(identity)
	if err != nil {
		return "", err
	}
	// The on-disk variables follow GitVersion JSON. The independent key format
	// prevents either implementation from replacing the other's cache entries.
	name := fmt.Sprintf("go-v1-%x", sha256.Sum256(input))
	return filepath.Join(repo.GitDirectory(), "gitversion_cache", name), nil
}

func loadCalculationCache(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var values map[string]any
	if err := decoder.Decode(&values); err != nil {
		return nil, err
	}
	if !json.Valid(data) {
		return nil, fmt.Errorf("invalid cache JSON")
	}
	if version, ok := values["FullSemVer"].(string); !ok || version == "" {
		return nil, fmt.Errorf("missing cache version")
	}
	// Earlier C# cache entries used the single version-source representation.
	if _, exists := values["CommitCountSourceDistance"]; !exists {
		values["CommitCountSourceDistance"] = values["VersionSourceDistance"]
	}
	if _, exists := values["CommitCountSourceSha"]; !exists {
		if sha, ok := values["VersionSourceSha"].(string); ok && sha != "" {
			values["CommitCountSourceSha"] = sha
		} else {
			values["CommitCountSourceSha"] = nil
		}
	}
	for _, name := range []string{"SemVerSourceIncrement", "SemVerSourceSemVer", "SemVerSourceSha"} {
		if _, exists := values[name]; !exists {
			values[name] = nil
		}
	}
	for _, name := range output.VariableNames {
		if _, ok := values[name]; !ok {
			return nil, fmt.Errorf("missing cached variable %s", name)
		}
		if n, ok := values[name].(json.Number); ok {
			number, err := n.Int64()
			if err != nil {
				return nil, err
			}
			values[name] = number
		}
	}
	return values, nil
}

func storeCalculationCache(path string, values map[string]any) error {
	data, err := output.MarshalVariables(values)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return writeFileAtomically(path, data, nil, true)
}
