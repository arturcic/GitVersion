package gitrepo

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/GitTools/GitVersion/go-prototype/internal/fixtures"
	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

type managedRepositoryCase struct {
	ID        string
	Arguments []json.RawMessage
}

func TestCSharpManagedRepository(t *testing.T) {
	var corpus struct {
		Revision string
		Suites   map[string]struct {
			Constants map[string]string
			Methods   map[string]struct{ Cases []managedRepositoryCase }
		}
	}
	data, err := os.ReadFile("../../testdata/csharp-managed-repository.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	if corpus.Revision != "14d222235b7dabb7033473528a76fd18478a9217" || len(corpus.Suites) != 3 {
		t.Fatal("original source inventory differs")
	}
	for name, suite := range corpus.Suites {
		t.Run(name, func(t *testing.T) {
			for method, spec := range suite.Methods {
				t.Run(method, func(t *testing.T) {
					for _, tc := range spec.Cases {
						t.Run(tc.ID, func(t *testing.T) {
							switch name {
							case "GitObjectIdTests":
								checkObjectID(t, method, tc, suite.Constants)
							case "GitRepositoryLayoutTests":
								checkRepositoryLayout(t, method)
							case "GitConfigurationFileTests":
								checkGitConfigurationFile(t, method)
							default:
								t.Fatal("unhandled original suite", name)
							}
						})
					}
				})
			}
		})
	}
}

func checkObjectID(t *testing.T, method string, tc managedRepositoryCase, constants map[string]string) {
	t.Helper()
	sha1, sha256 := constants["Sha1Hex"], constants["Sha256Hex"]
	parse := func(s string) ObjectID {
		id, err := parseObjectID(s)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	fromHex := func(s string) ObjectID {
		id, err := parseHexObjectID([]byte(s))
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	raw, err := hex.DecodeString(sha1)
	if err != nil {
		t.Fatal(err)
	}
	check := func(id ObjectID, length int, text string) {
		t.Helper()
		if int(id.length) != length || id.String() != text {
			t.Fatalf("want %d bytes and %s, got %d and %s", length, text, id.length, id)
		}
	}
	switch method {
	case "ParsesA40CharacterSha1HexString":
		check(parse(sha1), sha1Size, sha1)
	case "ParsesUpperCaseHexAndNormalizesToLowerCase":
		if got := parse(strings.ToUpper(sha1)).String(); got != sha1 {
			t.Fatal(got)
		}
	case "ParsesA64CharacterSha256HexString":
		check(parse(sha256), sha256Size, sha256)
	case "ParsesRawBytes":
		id, err := objectIDFromBytes(raw)
		if err != nil {
			t.Fatal(err)
		}
		check(id, sha1Size, sha1)
	case "ParsesAsciiEncodedHex":
		if got := fromHex(sha1).String(); got != sha1 {
			t.Fatal(got)
		}
	case "ParseRejectsInvalidHexLengths":
		var length int
		if err := json.Unmarshal(tc.Arguments[0], &length); err != nil {
			t.Fatal(err)
		}
		if _, err := parseObjectID(strings.Repeat("a", length)); !errors.Is(err, errObjectIDLength) {
			t.Fatal("wrong error", err)
		}
	case "ParseRejectsInvalidRawByteLengths":
		if _, err := objectIDFromBytes(make([]byte, 21)); !errors.Is(err, errObjectIDLength) {
			t.Fatal("wrong error", err)
		}
	case "ParseRejectsNonHexCharacters":
		_, err := parseObjectID("zz12736c27e40b389904d046dc63dc9f578117f4")
		var invalid hex.InvalidByteError
		if !errors.As(err, &invalid) {
			t.Fatal("wrong error", err)
		}
	case "EqualObjectIdsAreEqualAndHaveTheSameHashCode":
		left, right := parse(sha1), fromHex(sha1)
		if left != right || any(left) != any(right) || map[ObjectID]int{left: 1}[right] != 1 {
			t.Fatal("equal values differ")
		}
	case "DifferentObjectIdsAreNotEqual":
		if parse(sha1) == parse("4e912736c27e40b389904d046dc63dc9f5781180") {
			t.Fatal("different IDs compare equal")
		}
	case "ASha1IdIsNotEqualToASha256IdWithTheSamePrefix":
		if parse(sha1) == parse(sha1+strings.Repeat("0", 24)) {
			t.Fatal("hash width was lost")
		}
	case "CanBeUsedAsADictionaryKey":
		if got := map[ObjectID]string{parse(sha1): "value"}[fromHex(sha1)]; got != "value" {
			t.Fatal(got)
		}
	case "ToStringWithLengthReturnsAHexPrefix":
		var length int
		var expected string
		if err := json.Unmarshal(tc.Arguments[0], &length); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(tc.Arguments[1], &expected); err != nil {
			t.Fatal(err)
		}
		if got, err := parse(sha1).prefix(length); err != nil || got != expected {
			t.Fatalf("want %q, got %q, %v", expected, got, err)
		}
	case "ToStringWithInvalidLengthThrows":
		for _, length := range []int{41, -1} {
			if _, err := parse(sha1).prefix(length); !errors.Is(err, errObjectIDPrefix) {
				t.Fatal("wrong error", length, err)
			}
		}
	case "CopyToCopiesTheRawBytes":
		out := make([]byte, sha1Size)
		if err := parse(sha1).copyTo(out); err != nil || !bytes.Equal(out, raw) {
			t.Fatal(out, err)
		}
	case "AsUInt16ReturnsTheFirstTwoBytes":
		if got := parse(sha1).firstTwoBytes(); got != 0x4e91 {
			t.Fatal(got)
		}
	case "EmptyHasAZeroHashLength":
		var empty ObjectID
		if empty.length != 0 || empty != (ObjectID{}) {
			t.Fatal(empty)
		}
	default:
		t.Fatal("unhandled original method", method)
	}
}

func checkRepositoryLayout(t *testing.T, method string) {
	t.Helper()
	root := t.TempDir()
	if method == "ReturnsNullOutsideOfARepository" {
		for _, open := range []func(string) (repositoryLayout, error){openLayout, discoverLayout} {
			if got, err := open(root); !errors.Is(err, errRepositoryNotFound) || got != (repositoryLayout{}) {
				t.Fatal(got, err)
			}
		}
		return
	}
	if method == "RejectsSha256RepositoriesWithAClearMessage" || method == "ThrowsForReftableRepositories" {
		format, setting := "sha256", "objectformat = sha256"
		if method == "ThrowsForReftableRepositories" {
			format, setting = "reftable", "refstorage = reftable"
		}
		dir := filepath.Join(root, ".git")
		for _, name := range []string{"objects", "refs", format} {
			if err := os.MkdirAll(filepath.Join(dir, name), 0755); err != nil {
				t.Fatal(err)
			}
		}
		for name, value := range map[string]string{"config": "[core]\n repositoryformatversion = 1\n[extensions]\n " + setting + "\n", "HEAD": "ref: refs/heads/main\n"} {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(value), 0644); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := discoverLayout(root); !errors.Is(err, errUnsupportedRepositoryFormat) || !strings.Contains(err.Error(), format) {
			t.Fatal("wrong error", err)
		}
		return
	}
	steps := []fixtures.Step{{Op: "commit", ID: "root", Message: "a commit"}}
	if method == "DetectsShallowClones" {
		steps = []fixtures.Step{{Op: "commit", ID: "first", Message: "first"}, {Op: "commit", ID: "second", Message: "second"}}
	}
	steps = append(steps, fixtures.Step{Op: "checkpoint", ID: "head"})
	cases, err := fixtures.Generate([]fixtures.Recipe{{ID: "repository", InitialBranch: "main", Config: "workflow: GitFlow/v1\n", Steps: steps}}, filepath.Join(root, "fixtures"))
	if err != nil {
		t.Fatal(err)
	}
	repo, sha := cases[0].Path, cases[0].Head
	gitdir := filepath.Join(repo, ".git")
	switch method {
	case "DiscoversARepositoryFromItsRootAndFromANestedDirectory", "TryOpenDoesNotWalkUpToAnEnclosingRepository":
		nested := filepath.Join(repo, "sub", "dir")
		if err := os.MkdirAll(nested, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(nested, "file.txt"), []byte("content\n"), 0644); err != nil {
			t.Fatal(err)
		}
		if method == "TryOpenDoesNotWalkUpToAnEnclosingRepository" {
			if _, err := openLayout(nested); !errors.Is(err, errRepositoryNotFound) {
				t.Fatal("exact open walked up", err)
			}
			for _, path := range []string{repo, gitdir} {
				if _, err := openLayout(path); err != nil {
					t.Fatal(err)
				}
			}
			return
		}
		for _, path := range []string{repo, nested} {
			got, err := discoverLayout(path)
			if err != nil {
				t.Fatal(err)
			}
			shallow, err := got.isShallow()
			if got.gitDirectory != gitdir || got.commonDirectory != gitdir || got.workingDirectory != repo || got.objectsDirectory() != filepath.Join(gitdir, "objects") || shallow || err != nil {
				t.Fatal(got, shallow, err)
			}
		}
	case "ResolvesLinkedWorktrees":
		linked := filepath.Join(root, "linked")
		if err := fixtures.LinkWorktree(repo, linked, "wt-branch", false); err != nil {
			t.Fatal(err)
		}
		got, err := discoverLayout(linked)
		if err != nil {
			t.Fatal(err)
		}
		if got.workingDirectory != linked || !strings.Contains(got.gitDirectory, filepath.Join(".git", "worktrees")) || got.commonDirectory != gitdir {
			t.Fatal(got)
		}
		opened, err := Open(linked)
		if err != nil {
			t.Fatal(err)
		}
		branch, head := opened.Head()
		if branch != "refs/heads/wt-branch" || head != sha {
			t.Fatal(branch, head)
		}
		refs, err := opened.References()
		if err != nil {
			t.Fatal(err)
		}
		var names []string
		for _, r := range refs {
			if strings.HasPrefix(r.Name, "refs/heads/") {
				names = append(names, r.Name)
			}
		}
		if !slices.Equal(names, []string{"refs/heads/main", "refs/heads/wt-branch"}) {
			t.Fatal(names)
		}
		commit, err := opened.CommitMetadata(sha)
		if err != nil || commit.Message != "a commit\n" {
			t.Fatal(commit, err)
		}
	case "DetectsShallowClones", "DiscoversABareRepository":
		bare := method == "DiscoversABareRepository"
		path := filepath.Join(root, "clone")
		source, err := git.PlainOpen(repo)
		if err != nil {
			t.Fatal(err)
		}
		clone, err := git.PlainInitWithOptions(path, &git.PlainInitOptions{
			Bare: bare, InitOptions: git.InitOptions{DefaultBranch: plumbing.NewBranchReferenceName("main")},
		})
		if err != nil {
			t.Fatal(err)
		}
		tip := plumbing.NewHash(sha)
		objects, err := source.Storer.IterEncodedObjects(plumbing.AnyObject)
		if err != nil {
			t.Fatal(err)
		}
		defer objects.Close()
		if err := objects.ForEach(func(o plumbing.EncodedObject) error {
			// A depth-one clone has the tip commit and its tree, without
			// the parent commit object referenced by the tip.
			if !bare && o.Type() == plumbing.CommitObject && o.Hash() != tip {
				return nil
			}
			_, err := clone.Storer.SetEncodedObject(o)
			return err
		}); err != nil {
			t.Fatal(err)
		}
		if err := clone.Storer.SetReference(plumbing.NewHashReference(plumbing.NewBranchReferenceName("main"), tip)); err != nil {
			t.Fatal(err)
		}
		if !bare {
			if err := clone.Storer.SetShallow([]plumbing.Hash{tip}); err != nil {
				t.Fatal(err)
			}
		}
		got, err := discoverLayout(path)
		if err != nil {
			t.Fatal(err)
		}
		opened, err := got.open()
		if err != nil {
			t.Fatal(err)
		}
		if bare {
			if got.workingDirectory != "" || got.gitDirectory != path || got.commonDirectory != path {
				t.Fatal(got)
			}
			ref, err := opened.Reference(plumbing.NewBranchReferenceName("main"), true)
			if err != nil || ref.Hash().String() != sha {
				t.Fatal(ref, err)
			}
			return
		}
		shallow, err := got.isShallow()
		if err != nil || !shallow {
			t.Fatal("shallow boundary not detected", err)
		}
		ids, err := got.readShallowCommits()
		if err != nil || len(ids) != 1 || ids[0].String() != sha {
			t.Fatal(ids, err)
		}
		boundary, err := opened.CommitObject(plumbing.NewHash(sha))
		if err != nil || boundary.NumParents() != 1 {
			t.Fatal(boundary, err)
		}
		if _, err := opened.CommitObject(boundary.ParentHashes[0]); !errors.Is(err, plumbing.ErrObjectNotFound) {
			t.Fatal("parent must be absent", err)
		}
	default:
		t.Fatal("unhandled original method", method)
	}
}

func checkGitConfigurationFile(t *testing.T, method string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config")
	core := func(value string) string { return "[core]\n\tx = " + value + "\n" }
	remote := func(value string) string { return "[remote \"origin\"]\n\turl = " + value + "\n" }
	check := func(contents, section, sub, key, want string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
			t.Fatal(err)
		}
		got, found, err := gitConfigValue(path, section, sub, key)
		if err != nil || !found || got != want {
			t.Fatalf("want %q, got %q, found %t, error %v", want, got, found, err)
		}
	}
	switch method {
	case "UnescapesBackslashesInUnquotedValue":
		check(remote(`D:\\a\\_temp\\repo`), "remote", "origin", "url", `D:\a\_temp\repo`)
	case "UnescapesBackslashesInQuotedValue":
		check(remote(`"D:\\a\\repo"`), "remote", "origin", "url", `D:\a\repo`)
	case "KeepsForwardSlashPathUnchanged":
		check(remote("/srv/git/repositories/project.git"), "remote", "origin", "url", "/srv/git/repositories/project.git")
	case "StripsInlineCommentOutsideQuotesButNotInside":
		check(core("value ; trailing"), "core", "", "x", "value")
		check(core(`"a ; b"`), "core", "", "x", "a ; b")
	case "DecodesEscapeSequences":
		check(core(`a\tb\nc`), "core", "", "x", "a\tb\nc")
	case "TrimsUnquotedTrailingWhitespaceButKeepsQuotedWhitespace":
		check(core("value   "), "core", "", "x", "value")
		check(core(`"value   "`), "core", "", "x", "value   ")
	default:
		t.Fatal("unhandled original method", method)
	}
}
