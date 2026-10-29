package sitefs

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Store publishes site folders under workspace/sites and maps them to public URLs.
type Store struct {
	Workspace string
	BaseURL   string // e.g. https://kancil-tunnel.example.trycloudflare.com
}

func New(workspace, baseURL string) *Store {
	return &Store{Workspace: workspace, BaseURL: strings.TrimRight(baseURL, "/")}
}

var safeName = regexp.MustCompile(`^[a-zA-Z0-9._\-]+$`)

// Publish ensures the folder exists and returns its public URL.
func (s *Store) Publish(name string) (string, error) {
	if !safeName.MatchString(name) || name == "" {
		return "", fmt.Errorf("invalid site name %q", name)
	}
	dir := filepath.Join(s.Workspace, "sites", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	index := filepath.Join(dir, "index.html")
	if _, err := os.Stat(index); os.IsNotExist(err) {
		return "", fmt.Errorf("no index.html in sites/%s — write it first", name)
	}
	return fmt.Sprintf("%s/sites/%s/", s.BaseURL, name), nil
}
