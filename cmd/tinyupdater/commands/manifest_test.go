package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestManifestCommandGeneratesManifest(t *testing.T) {
	root := t.TempDir()
	git, calls := fixtureGitRunner(t, root, map[string]string{
		"tinyupdater.json": `{"files":["main.py","lib/util.py"]}`,
		"main.py":          "print('hello')\n",
		"lib/util.py":      "VALUE = 42\n",
	})
	updaterSource := "def check_for_updates():\n    return True\n"
	if err := os.WriteFile(filepath.Join(root, "updater.py"), []byte(updaterSource), 0600); err != nil {
		t.Fatal(err)
	}
	command := newManifestCommandWithGit(git, func() (string, error) {
		return root, nil
	})
	command.SetArgs([]string{"--repo", "owner/repo"})
	var output bytes.Buffer
	command.SetOut(&output)

	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}

	manifestData, err := os.ReadFile(filepath.Join(root, "repositories", "owner", "repo", "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var got manifest
	if err := json.Unmarshal(manifestData, &got); err != nil {
		t.Fatal(err)
	}
	want := manifest{
		Repository: "owner/repo",
		Version:    "a1b2c3d4",
		Files: []manifestFile{
			{File: "main.py", CRC32: fmt.Sprintf("%08x", crc32.ChecksumIEEE([]byte("print('hello')\n")))},
			{File: "lib/util.py", CRC32: fmt.Sprintf("%08x", crc32.ChecksumIEEE([]byte("VALUE = 42\n")))},
			{File: "lib/updater.py", CRC32: fmt.Sprintf("%08x", crc32.ChecksumIEEE([]byte(updaterSource)))},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("manifest = %#v, want %#v", got, want)
	}
	installedUpdater, err := os.ReadFile(filepath.Join(root, "repositories", "owner", "repo", "lib", "updater.py"))
	if err != nil {
		t.Fatal(err)
	}
	if string(installedUpdater) != updaterSource {
		t.Fatalf("installed updater = %q, want %q", installedUpdater, updaterSource)
	}
	if !strings.Contains(output.String(), "Generated manifest at ") {
		t.Fatalf("unexpected command output %q", output.String())
	}
	if !containsGitCall(*calls, "clone", "--branch", "main", "--single-branch", "https://github.com/owner/repo.git", filepath.Join(root, "repositories", "owner", "repo")) {
		t.Fatalf("clone command not found in calls: %v", *calls)
	}
}

func TestManifestCommandPullsExistingMain(t *testing.T) {
	root := t.TempDir()
	checkoutPath := filepath.Join(root, "repositories", "owner", "repo")
	if err := os.MkdirAll(checkoutPath, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "updater.py"), []byte("pass\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(checkoutPath, "tinyupdater.json"), []byte(`{"files":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	git, calls := fixtureGitRunner(t, root, nil)
	command := newManifestCommandWithGit(git, func() (string, error) {
		return root, nil
	})
	command.SetArgs([]string{"--repo", "owner/repo"})

	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}

	if !containsGitCall(*calls, "checkout", "main") {
		t.Fatalf("checkout main command not found in calls: %v", *calls)
	}
	if !containsGitCall(*calls, "pull", "--ff-only", "origin", "main") {
		t.Fatalf("pull main command not found in calls: %v", *calls)
	}
}

func TestManifestCommandReportsMissingConfiguration(t *testing.T) {
	root := t.TempDir()
	git, _ := fixtureGitRunner(t, root, nil)
	command := newManifestCommandWithGit(git, func() (string, error) {
		return root, nil
	})
	command.SetArgs([]string{"--repo", "owner/repo"})

	err := command.Execute()
	if err == nil || !strings.Contains(err.Error(), "tinyupdater.json not found") {
		t.Fatalf("Execute() error = %v, want missing-configuration error", err)
	}
}

func TestManifestCommandRejectsInvalidRepository(t *testing.T) {
	command := newManifestCommandWithGit(func(string, ...string) (string, error) {
		t.Fatal("git should not run for an invalid repository")
		return "", nil
	}, func() (string, error) {
		return t.TempDir(), nil
	})
	command.SetArgs([]string{"--repo", "../repo"})

	if err := command.Execute(); err == nil || !strings.Contains(err.Error(), "expected owner/repo") {
		t.Fatalf("Execute() error = %v, want invalid-repository error", err)
	}
}

func fixtureGitRunner(t *testing.T, root string, files map[string]string) (gitRunner, *[][]string) {
	t.Helper()
	calls := make([][]string, 0)
	run := func(dir string, args ...string) (string, error) {
		call := append([]string{dir}, args...)
		calls = append(calls, call)
		if len(args) == 0 {
			return "", fmt.Errorf("git called without arguments")
		}
		switch args[0] {
		case "clone":
			checkoutPath := args[len(args)-1]
			if err := os.MkdirAll(checkoutPath, 0755); err != nil {
				return "", err
			}
			for name, contents := range files {
				path := filepath.Join(checkoutPath, name)
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					return "", err
				}
				if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
					return "", err
				}
			}
			return "", nil
		case "rev-parse":
			if len(args) > 1 && args[1] == "--is-inside-work-tree" {
				return "true", nil
			}
			if len(args) > 1 && args[1] == "--short=8" {
				return "a1b2c3d4", nil
			}
		case "remote":
			return "https://github.com/owner/repo.git", nil
		case "checkout", "pull":
			return "", nil
		}
		return "", fmt.Errorf("unexpected git command: %v", args)
	}
	return run, &calls
}

func containsGitCall(calls [][]string, args ...string) bool {
	for _, call := range calls {
		if reflect.DeepEqual(call[1:], args) {
			return true
		}
	}
	return false
}
