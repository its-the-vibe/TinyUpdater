package commands

import (
	"encoding/json"
	"fmt"
	"hash/crc32"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

type gitRunner func(dir string, args ...string) (string, error)

type manifestConfig struct {
	Files []string `json:"files"`
}

type manifest struct {
	Repository string         `json:"repository"`
	Version    string         `json:"version"`
	Files      []manifestFile `json:"files"`
}

type manifestFile struct {
	File  string `json:"file"`
	CRC32 string `json:"crc32"`
}

func newManifestCommand() *cobra.Command {
	return newManifestCommandWithGit(runGit, os.Getwd)
}

func newManifestCommandWithGit(git gitRunner, getwd func() (string, error)) *cobra.Command {
	var repo string

	command := &cobra.Command{
		Use:   "manifest",
		Short: "Create or inspect an update manifest",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			owner, name, err := parseRepository(repo)
			if err != nil {
				return err
			}

			workingDirectory, err := getwd()
			if err != nil {
				return fmt.Errorf("get working directory: %w", err)
			}

			checkoutPath := filepath.Join(workingDirectory, "repositories", owner, name)
			if err := prepareRepository(repo, checkoutPath, git); err != nil {
				return err
			}

			if err := generateManifest(checkoutPath, name, git); err != nil {
				return err
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Generated manifest at %s\n", filepath.Join(checkoutPath, "manifest.json"))
			return err
		},
	}
	command.Flags().StringVar(&repo, "repo", "", "GitHub repository in owner/repo format")
	_ = command.MarkFlagRequired("repo")
	return command
}

func parseRepository(repo string) (string, string, error) {
	parts := strings.Split(repo, "/")
	if len(parts) != 2 || !validRepositoryComponent(parts[0]) || !validRepositoryComponent(parts[1]) {
		return "", "", fmt.Errorf("invalid repository %q: expected owner/repo", repo)
	}
	return parts[0], parts[1], nil
}

func validRepositoryComponent(component string) bool {
	if component == "" || component == "." || component == ".." {
		return false
	}
	for _, character := range component {
		if !(character >= 'a' && character <= 'z') &&
			!(character >= 'A' && character <= 'Z') &&
			!(character >= '0' && character <= '9') &&
			character != '-' && character != '_' && character != '.' {
			return false
		}
	}
	return true
}

func prepareRepository(repo, checkoutPath string, git gitRunner) error {
	info, err := os.Stat(checkoutPath)
	if os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(checkoutPath), 0755); err != nil {
			return fmt.Errorf("create repository directory: %w", err)
		}
		remote := "https://github.com/" + repo + ".git"
		if _, err := git("", "clone", "--branch", "main", "--single-branch", remote, checkoutPath); err != nil {
			return fmt.Errorf("clone %s: %w", repo, err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect repository directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("repository path %s is not a directory", checkoutPath)
	}

	if _, err := git(checkoutPath, "rev-parse", "--is-inside-work-tree"); err != nil {
		return fmt.Errorf("existing path %s is not a Git repository: %w", checkoutPath, err)
	}
	remote, err := git(checkoutPath, "remote", "get-url", "origin")
	if err != nil {
		return fmt.Errorf("read origin for %s: %w", repo, err)
	}
	if !githubRemoteMatches(remote, repo) {
		return fmt.Errorf("repository at %s does not have %s as its GitHub origin", checkoutPath, repo)
	}
	if _, err := git(checkoutPath, "checkout", "main"); err != nil {
		return fmt.Errorf("check out main for %s: %w", repo, err)
	}
	if _, err := git(checkoutPath, "pull", "--ff-only", "origin", "main"); err != nil {
		return fmt.Errorf("pull main for %s: %w", repo, err)
	}
	return nil
}

func githubRemoteMatches(remote, repo string) bool {
	remote = strings.TrimSpace(remote)
	var host, path string
	if strings.HasPrefix(remote, "git@") {
		userHost, remotePath, ok := strings.Cut(strings.TrimPrefix(remote, "git@"), ":")
		if !ok {
			return false
		}
		host, path = userHost, remotePath
	} else {
		parsed, err := url.Parse(remote)
		if err != nil {
			return false
		}
		host, path = parsed.Hostname(), parsed.Path
	}
	path = strings.TrimSuffix(strings.TrimPrefix(path, "/"), ".git")
	return (strings.EqualFold(host, "github.com") || strings.EqualFold(host, "ssh.github.com")) &&
		strings.EqualFold(path, repo)
}

func generateManifest(checkoutPath, repositoryName string, git gitRunner) error {
	configPath := filepath.Join(checkoutPath, "tinyupdater.json")
	configData, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("configuration file tinyupdater.json not found in %s", checkoutPath)
		}
		return fmt.Errorf("read %s: %w", configPath, err)
	}

	var config manifestConfig
	if err := json.Unmarshal(configData, &config); err != nil {
		return fmt.Errorf("parse %s: %w", configPath, err)
	}
	if config.Files == nil {
		return fmt.Errorf("%s must contain a files array", configPath)
	}

	resolvedRoot, err := filepath.EvalSymlinks(checkoutPath)
	if err != nil {
		return fmt.Errorf("resolve repository directory: %w", err)
	}
	files := make([]manifestFile, 0, len(config.Files))
	for _, name := range config.Files {
		localName := filepath.FromSlash(name)
		if !filepath.IsLocal(localName) || localName == "." {
			return fmt.Errorf("file path %q must be relative to the repository", name)
		}

		filePath := filepath.Join(checkoutPath, localName)
		resolvedPath, err := filepath.EvalSymlinks(filePath)
		if err != nil {
			return fmt.Errorf("resolve file %q: %w", name, err)
		}
		relativePath, err := filepath.Rel(resolvedRoot, resolvedPath)
		if err != nil || relativePath == ".." || strings.HasPrefix(relativePath, ".."+string(filepath.Separator)) {
			return fmt.Errorf("file path %q resolves outside the repository", name)
		}
		data, err := os.ReadFile(resolvedPath)
		if err != nil {
			return fmt.Errorf("read file %q: %w", name, err)
		}
		files = append(files, manifestFile{
			File:  name,
			CRC32: fmt.Sprintf("%08x", crc32.ChecksumIEEE(data)),
		})
	}

	version, err := git(checkoutPath, "rev-parse", "--short=8", "HEAD")
	if err != nil {
		return fmt.Errorf("read repository version: %w", err)
	}
	version = strings.TrimSpace(version)
	if len(version) < 7 || len(version) > 8 {
		return fmt.Errorf("Git returned an invalid short SHA %q", version)
	}
	for _, character := range version {
		if !(character >= '0' && character <= '9') && !(character >= 'a' && character <= 'f') {
			return fmt.Errorf("Git returned an invalid short SHA %q", version)
		}
	}

	data, err := json.MarshalIndent(manifest{Repository: repositoryName, Version: version, Files: files}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode manifest: %w", err)
	}
	data = append(data, '\n')

	manifestPath := filepath.Join(checkoutPath, "manifest.json")
	if info, err := os.Lstat(manifestPath); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("manifest path %s is a symlink", manifestPath)
	} else if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("inspect manifest path: %w", err)
	}
	if err := os.WriteFile(manifestPath, data, 0644); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}
	return nil
}

func runGit(dir string, args ...string) (string, error) {
	command := exec.Command("git", args...)
	if dir != "" {
		command.Dir = dir
	}
	output, err := command.Output()
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("%s: %w", strings.TrimSpace(string(exitError.Stderr)), err)
		}
		return "", err
	}
	return string(output), nil
}
