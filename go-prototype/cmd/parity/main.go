// parity is a Milestone A reference-capture and differential-test harness.
// It does not calculate versions. Candidate commands are supplied explicitly.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/GitTools/GitVersion/go-prototype/internal/fixtures"
	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
)

type Corpus struct {
	Schema         int                   `json:"schema"`
	RecipesSHA256  string                `json:"recipes_sha256"`
	BaselineSHA256 string                `json:"baseline_sha256"`
	Cases          []fixtures.Checkpoint `json:"cases"`
	Outputs        map[string]string     `json:"outputs_sha256"`
}
type CaseResult struct {
	ID          string              `json:"id"`
	Status      string              `json:"status"`
	Error       string              `json:"error,omitempty"`
	Differences []parity.Difference `json:"differences,omitempty"`
}
type Report struct {
	Mode       string       `json:"mode"`
	Calculator string       `json:"calculator"`
	Passed     int          `json:"passed"`
	Failed     int          `json:"failed"`
	Cases      []CaseResult `json:"cases"`
}

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: parity pin|fixtures|capture|verify|freeze|compare [options]")
	}
	f := flag.NewFlagSet(args[0], flag.ContinueOnError)
	specPath := f.String("spec", "baseline.json", "baseline specification")
	baselineDir := f.String("baseline", ".local/baseline", "published C# directory")
	lockPath := f.String("lock", ".local/baseline.lock.json", "baseline integrity lock")
	dotnet := f.String("dotnet", "", "absolute path to dotnet host")
	archive := f.String("archive", ".local/source.tar", "pinned source archive")
	recipesPath := f.String("recipes", "testdata/recipes.json", "fixture recipes")
	out := f.String("out", "", "new output directory (must not exist)")
	expected := f.String("expected", "", "reference capture directory, or JSON file for compare")
	candidatePath := f.String("candidate", "", "candidate command JSON; omitted means C# replay")
	actual := f.String("actual", "", "actual JSON file for compare")
	timeout := f.Duration("timeout", 30*time.Second, "timeout per executable invocation")
	if err := f.Parse(args[1:]); err != nil {
		return err
	}
	if f.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments")
	}
	switch args[0] {
	case "freeze":
		if *out == "" || *expected == "" {
			return fmt.Errorf("freeze requires --expected and --out")
		}
		return freeze(*expected, *specPath, *recipesPath, *out)
	case "compare":
		e, err := os.ReadFile(*expected)
		if err != nil {
			return err
		}
		a, err := os.ReadFile(*actual)
		if err != nil {
			return err
		}
		d, err := parity.Compare(e, a)
		if err != nil {
			return err
		}
		if len(d) > 0 {
			for _, v := range d {
				fmt.Printf("%s: %s (expected=%v actual=%v)\n", v.Field, v.Reason, v.Expected, v.Actual)
			}
			return fmt.Errorf("%d differences", len(d))
		}
		fmt.Println("JSON matches exactly")
		return nil
	case "pin":
		dir, err := filepath.Abs(*baselineDir)
		if err != nil {
			return err
		}
		b, err := parity.Pin(ctx, *specPath, *dotnet, dir, *archive, runtime.Version())
		if err != nil {
			return err
		}
		// Never replace an existing baseline lock implicitly.
		if err = writeNewJSON(*lockPath, b); err != nil {
			return err
		}
		fmt.Printf("Pinned %s (%s), %d published files\n", b.Spec.Revision, b.Spec.Environment["GITVERSION_GIT_BACKEND"], len(b.Files))
		return nil
	case "fixtures", "capture", "verify":
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
	if *out == "" {
		return fmt.Errorf("--out is required")
	}
	output, err := filepath.Abs(*out)
	if err != nil {
		return err
	}
	recipes, err := fixtures.Load(*recipesPath)
	if err != nil {
		return err
	}
	recipeHash, err := parity.FileHash(*recipesPath)
	if err != nil {
		return err
	}
	if args[0] == "fixtures" {
		checkpoints, err := fixtures.Generate(recipes, output)
		if err != nil {
			return err
		}
		if err = parity.WriteJSON(filepath.Join(output, "checkpoints.json"), checkpoints); err != nil {
			return err
		}
		fmt.Printf("Generated %d checkpoints\n", len(checkpoints))
		return nil
	}
	var baseline parity.Baseline
	if err = parity.ReadJSON(*lockPath, &baseline); err != nil {
		return err
	}
	dir, err := filepath.Abs(*baselineDir)
	if err != nil {
		return err
	}
	if err = parity.VerifyBaseline(baseline, *specPath, *dotnet, dir); err != nil {
		return err
	}
	lockHash, err := parity.FileHash(*lockPath)
	if err != nil {
		return err
	}
	command := parity.Command{Executable: *dotnet, Arguments: append([]string{"--fx-version", baseline.Spec.Runtime, filepath.Join(dir, "gitversion.dll")}, baseline.Spec.Arguments...)}
	calculator := "csharp-reference-replay"
	if args[0] == "capture" {
		calculator = "csharp-reference-capture"
	}
	if *candidatePath != "" {
		if args[0] != "verify" {
			return fmt.Errorf("--candidate requires verify")
		}
		if err = parity.ReadJSON(*candidatePath, &command); err != nil {
			return err
		}
		calculator = "external-candidate"
	}
	var reference Corpus
	if args[0] == "verify" {
		if err = parity.ReadJSON(filepath.Join(*expected, "corpus.json"), &reference); err != nil {
			return err
		}
		if reference.Schema != 1 || reference.RecipesSHA256 != recipeHash || reference.BaselineSHA256 != lockHash {
			return fmt.Errorf("reference corpus provenance differs")
		}
	}
	if err = os.Mkdir(output, 0755); err != nil {
		return err
	}
	fixturesRoot := filepath.Join(output, "fixtures")
	checkpoints, err := fixtures.Generate(recipes, fixturesRoot)
	if err != nil {
		return err
	}
	corpus := Corpus{Schema: 1, RecipesSHA256: recipeHash, BaselineSHA256: lockHash, Cases: checkpoints, Outputs: map[string]string{}}
	if args[0] == "verify" {
		if err = matchCases(reference.Cases, corpus.Cases); err != nil {
			return err
		}
		if len(reference.Outputs) != len(checkpoints) {
			return fmt.Errorf("reference output inventory differs")
		}
	}
	if args[0] == "capture" {
		if err = parity.WriteJSON(filepath.Join(output, "baseline.lock.json"), baseline); err != nil {
			return err
		}
	}
	report := Report{Mode: args[0], Calculator: calculator, Cases: []CaseResult{}}
	if err = validateConfigurations(ctx, checkpoints, baseline, *dotnet, dir, output, *timeout); err != nil {
		return err
	}
	for _, cp := range checkpoints {
		cr := CaseResult{ID: cp.ID, Status: "passed"}
		caseDir := filepath.Join(output, cp.ID)
		result, runErr := parity.Run(ctx, command, cp.Path, caseDir, baseline.Spec.Environment, *timeout)
		if runErr == nil {
			runErr = saveResult(caseDir, result)
		}
		if runErr == nil {
			runErr = result.Validate()
		}
		if runErr == nil && args[0] == "verify" {
			data, readErr := os.ReadFile(filepath.Join(*expected, cp.ID, "stdout.json"))
			runErr = readErr
			if runErr == nil {
				if parity.Hash(data) != reference.Outputs[cp.ID] {
					runErr = fmt.Errorf("reference output checksum differs")
				} else {
					cr.Differences, runErr = parity.Compare(data, result.Stdout)
				}
			}
			if runErr == nil && len(cr.Differences) > 0 {
				runErr = fmt.Errorf("%d differing fields", len(cr.Differences))
			}
		}
		if runErr == nil && cp.SemVer != "" {
			obj, _ := parity.Object(result.Stdout)
			if obj["SemVer"] != cp.SemVer {
				runErr = fmt.Errorf("independent SemVer assertion: expected %s got %v", cp.SemVer, obj["SemVer"])
			}
		}
		if runErr != nil {
			cr.Status = "failed"
			cr.Error = runErr.Error()
			report.Failed++
		} else {
			report.Passed++
			corpus.Outputs[cp.ID] = parity.Hash(result.Stdout)
		}
		report.Cases = append(report.Cases, cr)
		fmt.Printf("%s %s %s\n", strings.ToUpper(cr.Status), cp.ID, cr.Error)
	}
	// A partial or failed capture cannot be used as a reference corpus.
	if args[0] == "capture" && report.Failed == 0 {
		if err = parity.WriteJSON(filepath.Join(output, "corpus.json"), corpus); err != nil {
			return err
		}
	}
	if err = parity.WriteJSON(filepath.Join(output, "report.json"), report); err != nil {
		return err
	}
	if report.Failed > 0 {
		return fmt.Errorf("%d passed, %d failed; see %s/report.json", report.Passed, report.Failed, output)
	}
	fmt.Printf("%d/%d passed (%s)\n", report.Passed, len(checkpoints), calculator)
	return nil
}

func saveResult(dir string, r parity.Result) error {
	for name, data := range map[string][]byte{"stdout.json": r.Stdout, "stderr.log": r.Stderr} {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0644); err != nil {
			return err
		}
	}
	return parity.WriteJSON(filepath.Join(dir, "result.json"), r)
}

func validateConfigurations(ctx context.Context, cases []fixtures.Checkpoint, b parity.Baseline, dotnet, baselineDir, out string, timeout time.Duration) error {
	root := filepath.Join(out, "configurations")
	if err := os.Mkdir(root, 0755); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, cp := range cases {
		hash, err := parity.FileHash(filepath.Join(cp.Path, "GitVersion.yml"))
		if err != nil {
			return err
		}
		if seen[hash] {
			continue
		}
		seen[hash] = true
		prefix := []string{"--fx-version", b.Spec.Runtime, filepath.Join(baselineDir, "gitversion.dll"), "{repo}"}
		validation := parity.Command{Executable: dotnet, Arguments: append(append([]string{}, prefix...), "config", "validate", "--config", "GitVersion.yml", "--format", "json", "--telemetry-opt-out")}
		dir := filepath.Join(root, hash+"-validate")
		r, err := parity.Run(ctx, validation, cp.Path, dir, b.Spec.Environment, timeout)
		if err != nil {
			return err
		}
		if err = saveResult(dir, r); err != nil {
			return err
		}
		if err = r.Validate(); err != nil {
			return fmt.Errorf("configuration %s: %w", hash, err)
		}
		obj, _ := parity.Object(r.Stdout)
		if obj["valid"] != true || obj["configurationVersion"] != "v7" {
			return fmt.Errorf("reference did not validate v7 configuration %s", hash)
		}
		effective := parity.Command{Executable: dotnet, Arguments: append(append([]string{}, prefix...), "--config", "GitVersion.yml", "--show-config", "--no-cache", "--no-fetch", "--no-normalize", "--telemetry-opt-out")}
		dir = filepath.Join(root, hash+"-effective")
		r, err = parity.Run(ctx, effective, cp.Path, dir, b.Spec.Environment, timeout)
		if err != nil {
			return err
		}
		if err = os.WriteFile(filepath.Join(dir, "effective-config.yaml"), r.Stdout, 0644); err != nil {
			return err
		}
		if err = os.WriteFile(filepath.Join(dir, "stderr.log"), r.Stderr, 0644); err != nil {
			return err
		}
		if err = parity.WriteJSON(filepath.Join(dir, "result.json"), r); err != nil {
			return err
		}
		if r.ExitCode != 0 || r.Error != "" || len(r.Stdout) == 0 {
			return fmt.Errorf("effective configuration failed for %s: %s", hash, r.Error)
		}
	}
	return nil
}
