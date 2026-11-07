package server

import (
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/its-the-vibe/TinyUpdater/internal/config"
)

type manifest struct {
	Files []manifestFile `json:"files"`
}

type manifestFile struct {
	File string `json:"file"`
}

func NewHandler(cfg config.Config) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	})
	mux.HandleFunc("GET /files", func(w http.ResponseWriter, r *http.Request) {
		serveFile(w, r, cfg)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintln(w, "TinyUpdater server is running")
	})
	return mux
}

func NewHTTPServer(cfg config.Config) *http.Server {
	return &http.Server{
		Addr:              cfg.Server.Address,
		Handler:           NewHandler(cfg),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
}

func serveFile(w http.ResponseWriter, r *http.Request, cfg config.Config) {
	repository := r.URL.Query().Get("repo")
	name := r.URL.Query().Get("file")
	if repository == "" || name == "" {
		http.Error(w, "repo and file query parameters are required", http.StatusBadRequest)
		return
	}

	parts := strings.Split(repository, "/")
	if len(parts) != 2 || !validRepositoryComponent(parts[0]) || !validRepositoryComponent(parts[1]) {
		http.Error(w, "invalid repository", http.StatusBadRequest)
		return
	}
	allowed := false
	for _, candidate := range cfg.AllowedRepositories {
		if candidate == repository {
			allowed = true
			break
		}
	}
	if !allowed {
		http.Error(w, "repository is not allowed", http.StatusForbidden)
		return
	}

	repositoryRoot, err := filepath.EvalSymlinks(cfg.RepositoryPath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	repositoryPath := filepath.Join(cfg.RepositoryPath, parts[0], parts[1])
	repositoryPath, err = filepath.EvalSymlinks(repositoryPath)
	if err != nil || !isWithin(repositoryRoot, repositoryPath) {
		http.NotFound(w, r)
		return
	}
	info, err := os.Stat(repositoryPath)
	if err != nil || !info.IsDir() {
		http.NotFound(w, r)
		return
	}

	manifestData, err := readRepositoryFile(repositoryPath, "manifest.json")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if name != "manifest.json" {
		localName := filepath.FromSlash(name)
		if !filepath.IsLocal(localName) || localName == "." || strings.Contains(name, "\\") {
			http.NotFound(w, r)
			return
		}

		var listed manifest
		if err := json.Unmarshal(manifestData, &listed); err != nil {
			http.Error(w, "invalid repository manifest", http.StatusInternalServerError)
			return
		}
		found := false
		for _, file := range listed.Files {
			if file.File == name {
				found = true
				break
			}
		}
		if !found {
			http.NotFound(w, r)
			return
		}
	}

	data := manifestData
	if name != "manifest.json" {
		data, err = readRepositoryFile(repositoryPath, filepath.FromSlash(name))
		if err != nil {
			http.NotFound(w, r)
			return
		}
	}
	contentType := mime.TypeByExtension(filepath.Ext(name))
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
	fmt.Printf("Served file: %s from repository: %s\n", name, repository)
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

func readRepositoryFile(repositoryPath, name string) ([]byte, error) {
	localName := filepath.FromSlash(name)
	if !filepath.IsLocal(localName) || localName == "." {
		return nil, os.ErrNotExist
	}
	path := filepath.Join(repositoryPath, localName)
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return nil, os.ErrNotExist
	}
	resolvedPath, err := filepath.EvalSymlinks(path)
	if err != nil || !isWithin(repositoryPath, resolvedPath) {
		return nil, os.ErrNotExist
	}
	return os.ReadFile(resolvedPath)
}

func isWithin(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
