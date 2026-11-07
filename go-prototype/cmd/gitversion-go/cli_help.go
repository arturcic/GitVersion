package main

import (
	"fmt"
	"io"
	"runtime/debug"
	"slices"
	"strings"

	"github.com/GitTools/GitVersion/go-prototype/internal/commandline"
)

func cliInformationalVersion() string {
	if informationalVersion != "0.0.0" {
		return informationalVersion
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return informationalVersion
	}
	if info.Main.Version != "" && info.Main.Version != "(devel)" {
		return strings.TrimPrefix(info.Main.Version, "v")
	}
	for _, setting := range info.Settings {
		if setting.Key == "vcs.revision" && setting.Value != "" {
			return informationalVersion + "+" + setting.Value
		}
	}
	return informationalVersion
}

func containsHelpOption(args []string) bool {
	return slices.ContainsFunc(args, func(token string) bool {
		name, _ := splitSchemaOption(commandline.UnquoteText(token))
		return name == "--help" || name == "-h" || name == "-?"
	})
}

type helpEntry struct{ name, description string }

func writeCLIHelp(out io.Writer, path []cliSchemaCommand) error {
	command := path[len(path)-1]
	var usage []string
	var arguments, options, commands []helpEntry
	for _, node := range path {
		usage = append(usage, node.Name)
		for _, arg := range node.Arguments {
			name := "<" + arg.Name + ">"
			arguments = append(arguments, helpEntry{name, arg.Description})
			if !arg.Required {
				name = "[" + name + "]"
			}
			usage = append(usage, name)
		}
	}
	if len(command.Commands) != 0 {
		usage = append(usage, "[command]")
	}
	usage = append(usage, "[options]")

	addOption := func(option cliSchemaOption) {
		if option.Hidden {
			return
		}
		var names []string
		for _, alias := range option.Aliases {
			if strings.HasPrefix(alias, "-") {
				names = append(names, alias)
			}
		}
		names = append(names, option.Name)
		name := strings.Join(names, ", ")
		for _, arg := range option.Arguments {
			if !slices.Equal(arg.AcceptedValues, []string{"true", "false"}) {
				name += " <" + arg.Name + ">"
			}
		}
		options = append(options, helpEntry{name, option.Description})
	}
	for _, option := range command.Options {
		if option.Name != "--help" && option.Name != "--version" {
			addOption(option)
		}
	}
	for _, option := range path[0].Options {
		if len(path) > 1 && option.Recursive ||
			len(path) == 1 && (option.Name == "--help" || option.Name == "--version") {
			addOption(option)
		}
	}
	for _, child := range command.Commands {
		name := child.Name
		for _, arg := range child.Arguments {
			name += " <" + arg.Name + ">"
		}
		commands = append(commands, helpEntry{name, child.Description})
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Description:\n  %s\n\nUsage:\n  %s\n", command.Description, strings.Join(usage, " "))
	writeHelpSection(&b, "Arguments", arguments)
	writeHelpSection(&b, "Options", options)
	writeHelpSection(&b, "Commands", commands)
	b.WriteByte('\n')
	_, err := io.WriteString(out, b.String())
	return err
}

func writeHelpSection(b *strings.Builder, title string, entries []helpEntry) {
	if len(entries) == 0 {
		return
	}
	fmt.Fprintf(b, "\n%s:\n", title)
	width := 0
	for _, entry := range entries {
		width = max(width, len(entry.name))
	}
	for _, entry := range entries {
		lines := strings.Split(entry.description, "\n")
		fmt.Fprintf(b, "  %-*s  %s\n", width, entry.name, lines[0])
		for _, line := range lines[1:] {
			fmt.Fprintf(b, "%s%s\n", strings.Repeat(" ", width+4), line)
		}
	}
}
