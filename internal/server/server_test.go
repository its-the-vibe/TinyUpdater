package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/its-the-vibe/TinyUpdater/internal/config"
)

func TestHealthEndpoint(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	response := httptest.NewRecorder()

	NewHandler(config.Config{}).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if response.Body.String() != "ok\n" {
		t.Fatalf("body = %q, want %q", response.Body.String(), "ok\n")
	}
}

func TestFilesEndpoint(t *testing.T) {
	tests := []struct {
		name       string
		repository string
		file       string
		allowed    []string
		setup      func(string) error
		wantStatus int
		wantType   string
		wantBody   string
	}{
		{
			name:       "serves manifest",
			repository: "owner/repo",
			file:       "manifest.json",
			allowed:    []string{"owner/repo"},
			setup:      createRepository,
			wantStatus: http.StatusOK,
			wantType:   "application/json",
			wantBody:   `{"files":[{"file":"main.py"},{"file":"./main.py"}]}`,
		},
		{
			name:       "serves listed file",
			repository: "owner/repo",
			file:       "main.py",
			allowed:    []string{"owner/repo"},
			setup:      createRepository,
			wantStatus: http.StatusOK,
			wantBody:   "print('hello')",
		},
		{
			name:       "serves safe noncanonical relative file",
			repository: "owner/repo",
			file:       "./main.py",
			allowed:    []string{"owner/repo"},
			setup:      createRepository,
			wantStatus: http.StatusOK,
			wantBody:   "print('hello')",
		},
		{
			name:       "missing parameters",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid repository",
			repository: "../repo",
			file:       "manifest.json",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "repository is not allowed",
			repository: "owner/repo",
			file:       "manifest.json",
			allowed:    []string{"other/repo"},
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "repository does not exist",
			repository: "owner/repo",
			file:       "manifest.json",
			allowed:    []string{"owner/repo"},
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "manifest does not exist",
			repository: "owner/repo",
			file:       "manifest.json",
			allowed:    []string{"owner/repo"},
			setup: func(root string) error {
				return os.MkdirAll(filepath.Join(root, "owner", "repo"), 0755)
			},
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "file is not listed",
			repository: "owner/repo",
			file:       "private.txt",
			allowed:    []string{"owner/repo"},
			setup:      createRepository,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "listed file does not exist",
			repository: "owner/repo",
			file:       "main.py",
			allowed:    []string{"owner/repo"},
			setup: func(root string) error {
				repositoryPath := filepath.Join(root, "owner", "repo")
				if err := os.MkdirAll(repositoryPath, 0755); err != nil {
					return err
				}
				return os.WriteFile(filepath.Join(repositoryPath, "manifest.json"), []byte(`{"files":[{"file":"main.py"}]}`), 0600)
			},
			wantStatus: http.StatusNotFound,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			if test.setup != nil {
				if err := test.setup(root); err != nil {
					t.Fatal(err)
				}
			}
			cfg := config.Config{
				AllowedRepositories: test.allowed,
				RepositoryPath:      root,
			}
			request := httptest.NewRequest(http.MethodGet, "/files?repo="+test.repository+"&file="+test.file, nil)
			response := httptest.NewRecorder()

			NewHandler(cfg).ServeHTTP(response, request)

			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d; body = %q", response.Code, test.wantStatus, response.Body.String())
			}
			if test.wantType != "" && response.Header().Get("Content-Type") != test.wantType {
				t.Errorf("Content-Type = %q, want %q", response.Header().Get("Content-Type"), test.wantType)
			}
			if response.Code == http.StatusOK && response.Header().Get("Content-Type") == "" {
				t.Error("successful response has no Content-Type")
			}
			if test.wantBody != "" && response.Body.String() != test.wantBody {
				t.Errorf("body = %q, want %q", response.Body.String(), test.wantBody)
			}
		})
	}
}

func createRepository(root string) error {
	repositoryPath := filepath.Join(root, "owner", "repo")
	if err := os.MkdirAll(repositoryPath, 0755); err != nil {
		return err
	}
	manifestData := []byte(`{"files":[{"file":"main.py"},{"file":"./main.py"}]}`)
	if err := os.WriteFile(filepath.Join(repositoryPath, "manifest.json"), manifestData, 0600); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(repositoryPath, "main.py"), []byte("print('hello')"), 0600)
}
