package config

import (
	"bytes"
	"fmt"
	"gopkg.in/yaml.v3"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

var ConfigurationFileNames = []string{"GitVersion.yml", "GitVersion.yaml", ".GitVersion.yml", ".GitVersion.yaml"}

type ConfigurationFS interface {
	DirectoryExists(string) bool
	Files(string) ([]string, error)
	FileExists(string) bool
	ReadFile(string) ([]byte, error)
}
type OSConfigurationFS struct{}

func (OSConfigurationFS) DirectoryExists(p string) bool {
	info, e := os.Stat(p)
	return e == nil && info.IsDir()
}
func (OSConfigurationFS) FileExists(p string) bool {
	info, e := os.Stat(p)
	return e == nil && !info.IsDir()
}
func (OSConfigurationFS) Files(p string) ([]string, error) {
	entries, e := os.ReadDir(p)
	if e != nil {
		return nil, e
	}
	out := []string{}
	for _, entry := range entries {
		if !entry.IsDir() {
			out = append(out, filepath.Join(p, entry.Name()))
		}
	}
	return out, nil
}
func (OSConfigurationFS) ReadFile(p string) ([]byte, error) {
	info, e := os.Stat(p)
	if e != nil {
		return nil, e
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("unsupported nonregular config %s", filepath.Base(p))
	}
	if info.Size() > 1<<20 {
		return nil, fmt.Errorf("configuration exceeds 1 MiB")
	}
	return os.ReadFile(p)
}

type FileLocator struct {
	FS     ConfigurationFS
	Custom *string
	Log    func(string)
}

func (l FileLocator) fs() ConfigurationFS {
	if l.FS != nil {
		return l.FS
	}
	return OSConfigurationFS{}
}
func (l FileLocator) log(s string) {
	if l.Log != nil {
		l.Log(s)
	}
}
func pathValue(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
func configPath(base, file string) string {
	if filepath.IsAbs(file) {
		return filepath.Clean(file)
	}
	return filepath.Join(base, file)
}
func (l FileLocator) Get(directory *string) (*string, error) {
	dir := pathValue(directory)
	custom := pathValue(l.Custom)
	if strings.TrimSpace(custom) != "" {
		p := custom
		if strings.TrimSpace(dir) != "" {
			p = configPath(dir, custom)
		}
		if l.fs().FileExists(p) {
			l.log("Found configuration file at '" + p + "'")
			return &p, nil
		}
	}
	if strings.TrimSpace(dir) == "" || !l.fs().DirectoryExists(dir) {
		return nil, nil
	}
	files, e := l.fs().Files(dir)
	if e != nil {
		return nil, e
	}
	files = append([]string{}, files...)
	sort.Strings(files)
	for _, name := range ConfigurationFileNames {
		for _, file := range files {
			if strings.EqualFold(filepath.Base(file), name) {
				l.log("Found configuration file at '" + file + "'")
				return &file, nil
			}
		}
	}
	return nil, nil
}
func (l FileLocator) Verify(working, root string) error {
	if filepath.IsAbs(pathValue(l.Custom)) {
		return nil
	}
	a, e := filepath.Abs(working)
	if e != nil {
		return e
	}
	b, e := filepath.Abs(root)
	if e != nil {
		return e
	}
	equal := a == b
	if runtime.GOOS != "linux" {
		equal = strings.EqualFold(a, b)
	}
	if equal {
		return nil
	}
	w, e := l.Get(&working)
	if e != nil {
		return e
	}
	r, e := l.Get(&root)
	if e != nil {
		return e
	}
	if w != nil && r != nil {
		return fmt.Errorf("Ambiguous configuration file selection from '%s' and '%s'", *w, *r)
	}
	if w != nil || r != nil {
		return nil
	}
	for _, name := range ConfigurationFileNames {
		if strings.EqualFold(name, pathValue(l.Custom)) {
			return nil
		}
	}
	return fmt.Errorf("The configuration file was not found at '%s' or '%s'", configPath(working, pathValue(l.Custom)), configPath(root, pathValue(l.Custom)))
}
func ParseProvided(data []byte, version string) (Config, error) {
	selected, e := SelectConfigurationVersion(&version)
	if e != nil {
		return Config{}, e
	}
	if selected == "v6" {
		return ParseSelected(data, version)
	}
	var doc yaml.Node
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	err := decoder.Decode(&doc)
	if err == io.EOF {
		return Parse([]byte("workflow: GitFlow/v1"))
	}
	if err != nil {
		return Config{}, err
	}
	var extra yaml.Node
	if e := decoder.Decode(&extra); e != io.EOF {
		return Config{}, fmt.Errorf("multiple configuration documents are unsupported")
	}
	if len(doc.Content) == 1 {
		node := doc.Content[0]
		if node.Kind == yaml.ScalarNode && node.Tag == "!!null" {
			return Parse([]byte("workflow: GitFlow/v1"))
		}
		if node.Kind == yaml.MappingNode {
			found := false
			for i := 0; i < len(node.Content); i += 2 {
				if node.Content[i].Value == "workflow" {
					found = true
					if node.Content[i+1].Tag == "!!null" {
						node.Content[i+1] = &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "GitFlow/v1"}
					}
				}
			}
			if !found {
				node.Content = append([]*yaml.Node{{Kind: yaml.ScalarNode, Tag: "!!str", Value: "workflow"}, {Kind: yaml.ScalarNode, Tag: "!!str", Value: "GitFlow/v1"}}, node.Content...)
			}
		}
	}
	normalized, e := yaml.Marshal(&doc)
	if e != nil {
		return Config{}, e
	}
	return Parse(normalized)
}
func (l FileLocator) ProvideForDirectory(directory *string, version string) (Config, error) {
	file, e := l.Get(directory)
	if e != nil {
		return Config{}, e
	}
	if file == nil {
		l.log("No configuration file found, using default configuration")
		return ParseProvided(nil, version)
	}
	data, e := l.fs().ReadFile(*file)
	if e != nil {
		return Config{}, e
	}
	return ParseProvided(data, version)
}
func LoadNamed(repo, custom string) (Config, error) {
	return LoadNamedWithWarning(repo, custom, nil)
}

func LoadNamedWithWarning(repo, custom string, warn func(string)) (Config, error) {
	var name *string
	if custom != "" {
		name = &custom
	}
	locator := FileLocator{Custom: name}
	if name != nil {
		path := configPath(repo, custom)
		if !locator.fs().FileExists(path) {
			return Config{}, fmt.Errorf("Could not find config file at '%s'", path)
		}
	}
	version := os.Getenv("GITVERSION_CONFIGURATION_VERSION")
	file, err := locator.Get(&repo)
	if err != nil {
		return Config{}, err
	}
	if file == nil {
		return ParseProvided(nil, version)
	}
	data, err := locator.fs().ReadFile(*file)
	if err != nil {
		return Config{}, err
	}
	configuration, err := ParseProvided(data, version)
	if err == nil && warn != nil && IsExplicitV6(&version) {
		warn(LegacyConfigurationWarning(*file))
	}
	return configuration, err
}
