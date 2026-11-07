package main

import (
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/GitTools/GitVersion/go-prototype/internal/commandline"
)

const schemaUnbounded = 100000

// Set by release builds to the Go executable's informational version.
var informationalVersion = "0.0.0"

type cliSchemaMetadata struct {
	Name  string `json:"name"`
	Value any    `json:"value"`
}

type cliSchemaArity struct {
	Minimum int  `json:"minimum"`
	Maximum *int `json:"maximum,omitempty"`
}

type cliSchemaArgument struct {
	Name           string              `json:"name"`
	Description    string              `json:"description,omitempty"`
	Hidden         bool                `json:"hidden"`
	Required       bool                `json:"required"`
	Arity          cliSchemaArity      `json:"arity"`
	AcceptedValues []string            `json:"acceptedValues,omitempty"`
	Metadata       []cliSchemaMetadata `json:"metadata"`
}

type cliSchemaOption struct {
	Name        string              `json:"name"`
	Aliases     []string            `json:"aliases"`
	Description string              `json:"description,omitempty"`
	Hidden      bool                `json:"hidden"`
	Required    bool                `json:"required"`
	Recursive   bool                `json:"recursive"`
	Arguments   []cliSchemaArgument `json:"arguments"`
	Metadata    []cliSchemaMetadata `json:"metadata"`
}

type cliSchemaCommand struct {
	Name        string              `json:"name"`
	Aliases     []string            `json:"aliases"`
	Description string              `json:"description,omitempty"`
	Hidden      bool                `json:"hidden"`
	Arguments   []cliSchemaArgument `json:"arguments"`
	Options     []cliSchemaOption   `json:"options"`
	Commands    []cliSchemaCommand  `json:"commands"`
	Metadata    []cliSchemaMetadata `json:"metadata,omitempty"`
}

type cliSchemaInfo struct {
	Title   string `json:"title"`
	Version string `json:"version"`
}

type cliSchemaConventions struct {
	GroupOptions    bool   `json:"groupOptions"`
	OptionSeparator string `json:"optionSeparator"`
}

type cliSchemaDocument struct {
	Opencli     string               `json:"opencli"`
	Info        cliSchemaInfo        `json:"info"`
	Command     cliSchemaCommand     `json:"command"`
	Conventions cliSchemaConventions `json:"conventions"`
}

func schemaArgument(name, description string, min, max int, hidden, hasDefault bool) cliSchemaArgument {
	a := cliSchemaArgument{
		Name: name, Description: description, Hidden: hidden, Required: min > 0 && !hasDefault,
		Arity: cliSchemaArity{Minimum: min}, Metadata: []cliSchemaMetadata{},
	}
	if max != schemaUnbounded {
		a.Arity.Maximum = &max
	}
	return a
}

func (a cliSchemaArgument) withChoices(values []string, insensitive, numeric bool) cliSchemaArgument {
	a.AcceptedValues = values
	a.Metadata = []cliSchemaMetadata{
		{"gitversion.caseInsensitiveValues", insensitive},
		{"gitversion.acceptedValuesExhaustive", !numeric},
	}
	if numeric {
		a.Metadata = append(a.Metadata, cliSchemaMetadata{"gitversion.acceptsNumericValues", true})
	}
	return a
}

func schemaOption(name, description string, aliases []string, argument *cliSchemaArgument, multiple bool) cliSchemaOption {
	o := cliSchemaOption{
		Name: name, Description: description, Aliases: append([]string{}, aliases...),
		Arguments: []cliSchemaArgument{},
		Metadata:  []cliSchemaMetadata{{"gitversion.allowMultipleArgumentsPerToken", multiple}},
	}
	slices.Sort(o.Aliases)
	if argument != nil {
		o.Arguments = append(o.Arguments, *argument)
	}
	return o
}

func schemaCommand(name, description string) cliSchemaCommand {
	return cliSchemaCommand{
		Name: name, Description: description, Aliases: []string{}, Arguments: []cliSchemaArgument{},
		Options: []cliSchemaOption{}, Commands: []cliSchemaCommand{},
	}
}

func writeCLISchema(out io.Writer, root cliSchemaCommand) error {
	root.Name = "gitversion"
	root.Metadata = []cliSchemaMetadata{{"gitversion.optionArgumentSeparators", []string{" ", "=", ":"}}}
	document := cliSchemaDocument{
		Opencli:     "0.1",
		Info:        cliSchemaInfo{Title: "GitVersion", Version: informationalVersion},
		Command:     root,
		Conventions: cliSchemaConventions{GroupOptions: true, OptionSeparator: " "},
	}
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(document); err != nil {
		return fmt.Errorf("Could not export the CLI schema: %w", err)
	}
	return nil
}

func containsCLISchemaOption(args []string) bool {
	return slices.ContainsFunc(args, func(arg string) bool {
		name, _ := splitSchemaOption(commandline.UnquoteText(arg))
		return name == "--cli-schema"
	})
}

func cliSchemaInvocation(args []string, root cliSchemaCommand) (bool, error) {
	command := root
	requested := false
	positionals := 0
	var unmatched string
	for i := 0; i < len(args); i++ {
		token := commandline.UnquoteText(args[i])
		if token == "--" {
			if i+1 < len(args) && unmatched == "" {
				unmatched = args[i+1]
			}
			break
		}
		next := slices.IndexFunc(command.Commands, func(child cliSchemaCommand) bool { return child.Name == token })
		if next >= 0 {
			command, positionals = command.Commands[next], 0
			continue
		}
		name, inline := splitSchemaOption(token)
		option, found := findSchemaOption(root, command, name)
		if found {
			if name == "--cli-schema" {
				requested = true
			}
			if inline || len(option.Arguments) == 0 || i+1 == len(args) {
				continue
			}
			argument := option.Arguments[0]
			for i+1 < len(args) {
				value := args[i+1]
				valueName, _ := splitSchemaOption(value)
				_, isOption := findSchemaOption(root, command, valueName)
				if value == "--" || (isOption && !argument.Required) ||
					slices.ContainsFunc(command.Commands, func(child cliSchemaCommand) bool { return child.Name == value }) {
					break
				}
				if slices.Equal(argument.AcceptedValues, []string{"true", "false"}) {
					if _, ok := optionBoolean(value); !ok {
						break
					}
				}
				i++
				if !option.Metadata[0].Value.(bool) {
					break
				}
			}
			continue
		}
		if positionals < len(command.Arguments) && !strings.HasPrefix(token, "-") {
			positionals++
		} else if unmatched == "" {
			unmatched = token
		}
	}
	if !requested {
		return false, nil
	}
	if unmatched != "" {
		return true, argumentError(unmatched)
	}
	return true, nil
}

func splitSchemaOption(token string) (name string, inline bool) {
	if i := strings.IndexAny(token, "=:"); i >= 0 {
		return token[:i], true
	}
	return token, false
}

func findSchemaOption(root, command cliSchemaCommand, name string) (cliSchemaOption, bool) {
	match := func(option cliSchemaOption) bool {
		return option.Name == name || slices.Contains(option.Aliases, name)
	}
	if i := slices.IndexFunc(command.Options, match); i >= 0 {
		return command.Options[i], true
	}
	if i := slices.IndexFunc(root.Options, func(option cliSchemaOption) bool { return option.Recursive && match(option) }); i >= 0 {
		return root.Options[i], true
	}
	return cliSchemaOption{}, false
}
