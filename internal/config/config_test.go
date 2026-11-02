package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("server:\n  address: 127.0.0.1:8080\nallowedRepositories:\n  - owner/repo\nrepositoryPath: /repositories\n"), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.Address != "127.0.0.1:8080" {
		t.Fatalf("server.address = %q, want %q", cfg.Server.Address, "127.0.0.1:8080")
	}
	if len(cfg.AllowedRepositories) != 1 || cfg.AllowedRepositories[0] != "owner/repo" {
		t.Fatalf("allowedRepositories = %#v, want [owner/repo]", cfg.AllowedRepositories)
	}
	if cfg.RepositoryPath != "/repositories" {
		t.Fatalf("repositoryPath = %q, want %q", cfg.RepositoryPath, "/repositories")
	}
}

func TestLoadRequiresServerAddress(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("server: {}\n"), 0600); err != nil {
		t.Fatal(err)
	}

	if _, err := Load(path); err == nil {
		t.Fatal("Load succeeded without server.address")
	}
}
