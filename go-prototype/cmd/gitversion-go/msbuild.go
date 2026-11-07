package main

import (
	"crypto/rand"
	"embed"
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
	const usage = "usage: gitversion-go msbuild generate-version-info|check-assembly-info --project-file PATH"
	if len(args) == 0 {
		return errors.New(usage)
	}
	flags := flag.NewFlagSet(args[0], flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	project := flags.String("project-file", "", "MSBuild project path")
	switch args[0] {
	case "check-assembly-info":
		if err := flags.Parse(args[1:]); err != nil || *project == "" {
			return errors.New(usage)
		}
		return checkMSBuildAssemblyInfo(*project, flags.Args())
	case "generate-version-info":
		language := flags.String("language", "C#", "C#, F#, or VB")
		intermediate := flags.String("intermediate-output-path", "", "generated file directory")
		useNamespace := flags.Bool("use-project-namespace", false, "use project namespace")
		rootNamespace := flags.String("root-namespace", "", "project root namespace")
		if err := flags.Parse(args[1:]); err != nil || *project == "" || flags.NArg() != 0 {
			return errors.New(usage)
		}
		path, extension, err := generatedVersionInfoPath(*project, *intermediate, *language, "GitVersionInformation")
		if err != nil {
			return err
		}
		projectDirectory := filepath.Dir(*project)
		agent := buildagent.Resolve(projectDirectory, os.LookupEnv)
		options := gitrepo.OpenOptions{NormalizeDetached: agent.Name != "LocalBuild"}
		if branch := agent.CurrentBranch(); branch != nil {
			options.BuildBranch = *branch
		}
		if tag := agent.CurrentTag(); tag != nil {
			options.BuildTag = *tag
		}
		values, _, err := calculateCLI(projectDirectory, "", "", "", nil, diagnostics, options)
		if err != nil {
			return err
		}
		var namespace string
		if *useNamespace {
			namespace = *rootNamespace
			if strings.TrimSpace(namespace) == "" {
				namespace = strings.TrimSuffix(filepath.Base(*project), filepath.Ext(*project))
			}
		}
		if err := generateVersionInfo(path, extension, namespace, values); err != nil {
			return err
		}
		_, err = fmt.Fprintln(out, path)
		return err
	default:
		return errors.New(usage)
	}
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
