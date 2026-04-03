package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadBackendConfigDefault(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	bc, err := LoadBackendConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bc.Backend.Type != "local" {
		t.Errorf("expected type %q, got %q", "local", bc.Backend.Type)
	}
}

func TestLoadBackendConfigLocal(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	path := filepath.Join(dir, "plr", "backend.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("backend:\n  type: local\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	bc, err := LoadBackendConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bc.Backend.Type != "local" {
		t.Errorf("expected type %q, got %q", "local", bc.Backend.Type)
	}
}

func TestLoadBackendConfigS3(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	path := filepath.Join(dir, "plr", "backend.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	content := `backend:
  type: s3
  bucket: my-bucket
  region: eu-west-1
  prefix: team/
  endpoint: http://localhost:9000
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	bc, err := LoadBackendConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bc.Backend.Type != "s3" {
		t.Errorf("type = %q", bc.Backend.Type)
	}
	if bc.Backend.Bucket != "my-bucket" {
		t.Errorf("bucket = %q", bc.Backend.Bucket)
	}
	if bc.Backend.Region != "eu-west-1" {
		t.Errorf("region = %q", bc.Backend.Region)
	}
	if bc.Backend.Prefix != "team/" {
		t.Errorf("prefix = %q", bc.Backend.Prefix)
	}
	if bc.Backend.Endpoint != "http://localhost:9000" {
		t.Errorf("endpoint = %q", bc.Backend.Endpoint)
	}
}

func TestLoadBackendConfigEmptyType(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	path := filepath.Join(dir, "plr", "backend.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("backend:\n  bucket: x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	bc, err := LoadBackendConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bc.Backend.Type != "local" {
		t.Errorf("expected default type %q, got %q", "local", bc.Backend.Type)
	}
}

func TestLoadBackendConfigInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	path := filepath.Join(dir, "plr", "backend.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{{bad"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadBackendConfig()
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}
