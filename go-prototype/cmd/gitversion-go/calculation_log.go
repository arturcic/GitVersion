package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/GitTools/GitVersion/go-prototype/internal/buildagent"
	"github.com/GitTools/GitVersion/go-prototype/internal/config"
	"github.com/GitTools/GitVersion/go-prototype/internal/semver"
)

type logLevel int

const (
	logVerbose logLevel = iota
	logDebug
	logInfo
	logWarning
	logError
)

var logNames = [...]string{"VERB", "DBUG", "INFO", "WARN", "EROR"}
var logURLPassword = regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9+.-]*://[^\s/:@]+:)[^\s/@]+@`)

const gitGraphHint = "Please run `git log --graph --format=\"%h %cr %d\" --decorate --date=relative --all --remotes=*` to see the git graph. This can help you troubleshoot any issues.\n"

type cliLog struct {
	indent  string
	stderr  io.Writer
	file    *os.File
	level   logLevel
	console bool
}

func newCLILog(path, verbosity string, buildServer bool, stderr io.Writer) *cliLog {
	log := &cliLog{stderr: stderr, level: logInfo, console: buildServer || strings.EqualFold(path, "console")}
	switch verbosity {
	case "Verbose":
		log.level = logVerbose
	case "Diagnostic":
		log.level = logDebug
	case "Minimal":
		log.level = logWarning
	case "Quiet":
		log.level = logError
	}
	if path != "" && !strings.EqualFold(path, "console") {
		// Serilog opens the file relative to the invocation directory and treats
		// a sink failure independently of the command's result.
		if os.MkdirAll(filepath.Dir(path), 0755) == nil {
			log.file, _ = os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
		}
	}
	return log
}

func (l *cliLog) close() {
	if l.file != nil {
		_ = l.file.Close()
	}
}

func (l *cliLog) write(level logLevel, message string) {
	l.writeSinks(level, message, message)
}

func (l *cliLog) taggedVersionLookup(prefix string, format semver.ParseMode, cached bool) {
	level, title := logInfo, "Getting tagged semantic versions."
	if cached {
		level, title = logDebug, "│   Returning cached tagged semantic versions."
	}
	message := fmt.Sprintf("%s TagPrefix: %s and Format: ", title, prefix)
	l.writeSinks(level, message+string(format), message+fmt.Sprintf("%q", format))
}

func (l *cliLog) writeSinks(level logLevel, message, fileMessage string) {
	if !l.enabled(level) {
		return
	}
	now := time.Now()
	prefix := fmt.Sprintf("%s [%s:%02d] ", logNames[level], now.Format("06-01-02 15:04:05"), now.Nanosecond()/10_000_000)
	render := func(text string) string {
		// The message template may contain literal LF. Only the sink's
		// record terminator follows Environment.NewLine.
		return prefix + l.indent + logURLPassword.ReplaceAllString(text, "${1}*******@") + string(nativeText([]byte{'\n'}))
	}
	if l.console || level == logWarning {
		_, _ = io.WriteString(rawTextWriter(l.stderr), render(message))
	}
	if l.file != nil {
		_, _ = io.WriteString(l.file, render(fileMessage))
	}
}

func (l *cliLog) initialize(directory, version string, agent *buildagent.Agent, diagnose bool) {
	l.featureSelection(version)
	l.write(logInfo, fmt.Sprintf("Applicable build agent found: '%s'.", agent.Name))
	if diagnose {
		l.write(logInfo, string(nativeText([]byte(gitGraphHint))))
	}
	if !(config.OSConfigurationFS{}).DirectoryExists(directory) {
		l.write(logWarning, fmt.Sprintf("The working directory '%s' does not exist.", directory))
		return
	}
	l.write(logInfo, "Working directory: "+directory)
}

func (l *cliLog) configLookup(message string) {
	level := logInfo
	if strings.HasPrefix(message, "Trying to find configuration file ") {
		level = logDebug
	} else if strings.Contains(message, "uses the temporary v6 compatibility mode.") {
		level = logWarning
	}
	l.write(level, message)
}

func (l *cliLog) featureSelection(version string) {
	l.write(logInfo, "Argument parser version: v7; Configuration version: "+version+"; Git backend: managed")
}

func (l *cliLog) enabled(level logLevel) bool {
	return level >= l.level && (l.file != nil || l.console || level == logWarning)
}

func (l *cliLog) indented() *cliLog {
	child := *l
	child.indent += "│   "
	return &child
}

func (l *cliLog) info() func(string) {
	if !l.enabled(logInfo) {
		return nil
	}
	return func(message string) { l.write(logInfo, message) }
}
