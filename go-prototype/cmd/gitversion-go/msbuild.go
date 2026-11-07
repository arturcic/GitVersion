package main

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/json"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/GitTools/GitVersion/go-prototype/internal/buildagent"
	"github.com/GitTools/GitVersion/go-prototype/internal/gitrepo"
	"github.com/GitTools/GitVersion/go-prototype/internal/output"
	"github.com/GitTools/GitVersion/go-prototype/internal/patterns"
)

//go:embed versioninfo/*/GitVersionInformation.*
var versionInfoTemplates embed.FS

func runMSBuild(args []string, out, diagnostics io.Writer) error {
	const usage = "usage: gitversion-go msbuild <generate-version-info|check-assembly-info|update-assembly-info|get-version|write-version-info> --project-file PATH"
	if len(args) == 0 {
		return errors.New(usage)
	}
	command := args[0]
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	project := flags.String("project-file", "", "MSBuild project path")
	var language, intermediate, rootNamespace, compileList, outputFile string
	var useNamespace bool
	versionFile := flags.String("version-file", "", "existing GitVersion JSON")
	switch command {
	case "check-assembly-info", "write-version-info":
	case "get-version":
		flags.StringVar(&outputFile, "output-file", "", "version properties XML path")
	case "generate-version-info", "update-assembly-info":
		flags.StringVar(&language, "language", "C#", "C#, F#, or VB")
		flags.StringVar(&intermediate, "intermediate-output-path", "", "generated file directory")
		if command == "generate-version-info" {
			flags.BoolVar(&useNamespace, "use-project-namespace", false, "use project namespace")
			flags.StringVar(&rootNamespace, "root-namespace", "", "project root namespace")
		} else {
			flags.StringVar(&compileList, "compile-list", "", "newline-delimited compile items")
		}
	default:
		return errors.New(usage)
	}
	if err := flags.Parse(args[1:]); err != nil || *project == "" {
		return errors.New(usage)
	}
	compileFiles := flags.Args()
	if command == "check-assembly-info" {
		return checkMSBuildAssemblyInfo(*project, compileFiles)
	}
	if command == "update-assembly-info" {
		if compileList != "" {
			raw, err := os.ReadFile(compileList)
			if err != nil {
				return err
			}
			for _, name := range strings.Split(assemblyInfoText(raw), "\n") {
				if name = strings.TrimSuffix(name, "\r"); name != "" {
					compileFiles = append(compileFiles, name)
				}
			}
		}
		if err := checkMSBuildAssemblyInfo(*project, compileFiles); err != nil {
			return err
		}
	} else if flags.NArg() != 0 {
		return errors.New(usage)
	}

	directory := filepath.Dir(*project)
	agent := buildagent.Resolve(directory, os.LookupEnv)
	values, err := msbuildVariables(directory, *versionFile, agent, diagnostics)
	if err != nil {
		return err
	}
	switch command {
	case "get-version":
		return writeMSBuildVersionXML(values, outputFile, out)
	case "write-version-info":
		update, err := updateBuildNumberSetting(directory, "", nil)
		if err != nil {
			return err
		}
		return agent.WriteIntegration(context.Background(), out, values, update)
	case "update-assembly-info":
		path, _, err := generatedVersionInfoPath(*project, intermediate, language, "AssemblyInfo")
		if err != nil {
			return err
		}
		if err := updateAssemblyInfo(filepath.Dir(path), []string{filepath.Base(path)}, true, values, diagnostics); err != nil {
			return err
		}
		_, err = fmt.Fprintln(out, path)
		return err
	default:
		path, extension, err := generatedVersionInfoPath(*project, intermediate, language, "GitVersionInformation")
		if err != nil {
			return err
		}
		var namespace string
		if useNamespace {
			namespace = rootNamespace
			if strings.TrimSpace(namespace) == "" {
				namespace = strings.TrimSuffix(filepath.Base(*project), filepath.Ext(*project))
			}
		}
		if err := generateVersionInfo(path, extension, namespace, values); err != nil {
			return err
		}
		_, err = fmt.Fprintln(out, path)
		return err
	}
}

func msbuildVariables(directory, versionFile string, agent *buildagent.Agent, diagnostics io.Writer) (map[string]any, error) {
	if versionFile != "" {
		data, err := os.ReadFile(resolveMigrationPath(directory, versionFile))
		if err != nil {
			return nil, err
		}
		decoder := json.NewDecoder(strings.NewReader(assemblyInfoText(data)))
		decoder.UseNumber()
		var values map[string]any
		if err := decoder.Decode(&values); err != nil {
			return nil, err
		}
		if values == nil {
			return nil, errors.New("expected GitVersion JSON object")
		}
		var extra any
		if err := decoder.Decode(&extra); err != io.EOF {
			return nil, errors.New("unexpected data after GitVersion JSON")
		}
		return values, nil
	}
	options := gitrepo.OpenOptions{NormalizeDetached: agent.Name != "LocalBuild"}
	if branch := agent.CurrentBranch(); branch != nil {
		options.BuildBranch = *branch
	}
	if tag := agent.CurrentTag(); tag != nil {
		options.BuildTag = *tag
	}
	values, _, err := calculateCLI(directory, "", "", "", nil, diagnostics, options)
	return values, err
}

func writeMSBuildVersionXML(values map[string]any, path string, out io.Writer) error {
	var content strings.Builder
	encoder := xml.NewEncoder(&content)
	root := xml.StartElement{Name: xml.Name{Local: "GitVersion"}}
	if err := encoder.EncodeToken(root); err != nil {
		return err
	}
	for _, name := range output.VariableNames {
		element := xml.StartElement{Name: xml.Name{Local: name}}
		if err := encoder.EncodeElement(buildagent.ValueText(values[name]), element); err != nil {
			return err
		}
	}
	if err := encoder.EncodeToken(root.End()); err != nil {
		return err
	}
	if err := encoder.Flush(); err != nil {
		return err
	}
	if path == "" {
		_, err := fmt.Fprintln(out, content.String())
		return err
	}
	info, err := os.Lstat(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return writeFileAtomically(path, []byte(content.String()), info, true)
}

func languageExtension(language string) (string, error) {
	switch language {
	case "C#":
		return "cs", nil
	case "F#":
		return "fs", nil
	case "VB":
		return "vb", nil
	default:
		return "", fmt.Errorf("Unknown language detected: '%s'", language)
	}
}

func generatedVersionInfoPath(project, intermediate, language, stem string) (string, string, error) {
	extension, err := languageExtension(language)
	if err != nil {
		return "", "", err
	}
	if intermediate == "" {
		projectName := strings.TrimSuffix(filepath.Base(project), filepath.Ext(project))
		intermediate = filepath.Join(os.TempDir(), "GitVersionTask")
		stem = stem + "_" + projectName + "_" + rand.Text()
	}
	return filepath.Join(intermediate, stem+".g."+extension), extension, nil
}

func generateVersionInfo(path, extension, namespace string, values map[string]any) error {
	template, err := versionInfoTemplates.ReadFile("versioninfo/templates/GitVersionInformation." + extension)
	if err != nil {
		return err
	}
	member, err := versionInfoTemplates.ReadFile("versioninfo/members/GitVersionInformation." + extension)
	if err != nil {
		return err
	}
	memberPattern := strings.TrimRight(string(member), "\r\n")
	var open, close, indent string
	memberIndent := "    "
	switch extension {
	case "cs":
		if namespace != "" {
			namespace = "\nnamespace " + namespace
			open, close, indent = "\n{", "\n}", "    "
			memberIndent += indent
		}
	case "fs":
		if namespace == "" {
			namespace = "global"
		}
	case "vb":
		if namespace == "" {
			namespace = "Global"
		}
		memberIndent += "    "
	}
	var members []string
	for _, name := range slices.Sorted(slices.Values(output.VariableNames)) {
		value := buildagent.ValueText(values[name])
		if extension == "vb" {
			// VB escapes a quote by doubling it.
			value = strings.ReplaceAll(value, `"`, `""`)
		} else {
			quoted := strconv.Quote(value)
			value = quoted[1 : len(quoted)-1]
		}
		members = append(members, memberIndent+strings.NewReplacer("{0}", name, "{1}", value).Replace(memberPattern))
	}
	text := strings.NewReplacer("{{", "{", "}}", "}", "{0}", strings.Join(members, "\n"),
		"{1}", namespace, "{2}", open, "{3}", close, "{4}", indent).Replace(string(template))
	if old, err := os.ReadFile(path); err == nil && assemblyInfoText(old) == text {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0777); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return writeFileAtomically(path, []byte(text), info, true)
}

func checkMSBuildAssemblyInfo(project string, compileFiles []string) error {
	for _, name := range compileFiles {
		if !strings.Contains(name, "AssemblyInfo") {
			continue
		}
		extension := filepath.Ext(name)
		var language string
		switch extension {
		case ".cs":
			language = "CSharp"
		case ".fs":
			language = "FSharp"
		case ".vb":
			language = "VisualBasic"
		default:
			return fmt.Errorf("File with name containing AssemblyInfo could not be checked for assembly version attributes which conflict with the attributes generated by GitVersion %s", name)
		}
		trivia, err := patterns.Builtin("AssemblyVersion." + language + ".TriviaRegex")
		if err != nil {
			return err
		}
		attribute, err := patterns.Builtin("AssemblyVersion." + language + ".AttributeRegex")
		if err != nil {
			return err
		}
		raw, err := os.ReadFile(resolveMigrationPath(filepath.Dir(project), name))
		if err != nil {
			return err
		}
		withoutTrivia, err := trivia.ReplaceEach(assemblyInfoText(raw)+"\n", func(match string) string {
			if strings.HasPrefix(match, "//") || strings.HasPrefix(match, "'") {
				return "\n"
			}
			return ""
		})
		if err != nil {
			return err
		}
		found, err := attribute.Find(withoutTrivia)
		if err != nil {
			return err
		}
		if found != nil {
			return fmt.Errorf("File contains assembly version attributes which conflict with the attributes generated by GitVersion %s", name)
		}
	}
	return nil
}
