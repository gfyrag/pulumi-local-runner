package store

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gfyrag/plr/internal/config"
)

func TestLocalStoreLoadNonExistent(t *testing.T) {
	s := NewLocalStore(t.TempDir())

	cfg, err := s.LoadConfig()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(cfg.Apps) != 0 {
		t.Fatalf("expected empty config, got %d apps", len(cfg.Apps))
	}
}

func TestLocalStoreRoundtrip(t *testing.T) {
	s := NewLocalStore(t.TempDir())

	original := &config.Config{
		Apps: []config.App{
			{
				Name: "myapp",
				Repo: "git@github.com:org/myapp.git",
				Path: "infra",
				Stacks: []config.Stack{
					{Name: "dev", Branch: "main"},
					{Name: "prod", Ref: "v1.0.0"},
				},
			},
		},
	}

	if err := s.SaveConfig(original); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := s.LoadConfig()
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if len(loaded.Apps) != 1 {
		t.Fatalf("expected 1 app, got %d", len(loaded.Apps))
	}
	app := loaded.Apps[0]
	if app.Name != "myapp" {
		t.Errorf("name = %q, want %q", app.Name, "myapp")
	}
	if app.Repo != "git@github.com:org/myapp.git" {
		t.Errorf("repo = %q", app.Repo)
	}
	if app.Path != "infra" {
		t.Errorf("path = %q, want %q", app.Path, "infra")
	}
	if len(app.Stacks) != 2 {
		t.Fatalf("expected 2 stacks, got %d", len(app.Stacks))
	}
	if app.Stacks[0].Branch != "main" {
		t.Errorf("stack[0].branch = %q", app.Stacks[0].Branch)
	}
	if app.Stacks[1].Ref != "v1.0.0" {
		t.Errorf("stack[1].ref = %q", app.Stacks[1].Ref)
	}
}

func TestLocalStoreDefaultPath(t *testing.T) {
	s := NewLocalStore(t.TempDir())

	cfg := &config.Config{
		Apps: []config.App{
			{
				Name:   "nopath",
				Repo:   "git@github.com:org/nopath.git",
				Stacks: []config.Stack{{Name: "dev", Branch: "main"}},
			},
		},
	}
	if err := s.SaveConfig(cfg); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := s.LoadConfig()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Apps[0].Path != "." {
		t.Errorf("expected default path %q, got %q", ".", loaded.Apps[0].Path)
	}
}

func TestLocalStoreInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	s := NewLocalStore(dir)

	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("{{invalid yaml"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := s.LoadConfig()
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestLocalStoreStackConfigRoundtrip(t *testing.T) {
	s := NewLocalStore(t.TempDir())

	data := []byte("config:\n  key: value\n")

	if err := s.WriteStackConfig("myapp", "dev", data); err != nil {
		t.Fatalf("write: %v", err)
	}

	got, err := s.ReadStackConfig("myapp", "dev")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != string(data) {
		t.Errorf("got %q, want %q", got, data)
	}
}

func TestLocalStoreStackConfigNonExistent(t *testing.T) {
	s := NewLocalStore(t.TempDir())

	data, err := s.ReadStackConfig("myapp", "dev")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if data != nil {
		t.Fatalf("expected nil data, got %q", data)
	}
}

func TestNewDefaultLocalStore(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	s, err := NewDefaultLocalStore()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if s.configDir != filepath.Join(dir, "plr") {
		t.Errorf("configDir = %q, want %q", s.configDir, filepath.Join(dir, "plr"))
	}
}
