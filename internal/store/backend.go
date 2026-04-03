package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gfyrag/plr/internal/config"
	"gopkg.in/yaml.v3"
)

// BackendConfig describes which store backend to use.
type BackendConfig struct {
	Backend BackendSpec `yaml:"backend"`
}

// BackendSpec is the backend section of the config.
type BackendSpec struct {
	Type     string `yaml:"type"`               // "local" (default) or "s3"
	Bucket   string `yaml:"bucket,omitempty"`    // S3 bucket name
	Region   string `yaml:"region,omitempty"`    // AWS region
	Prefix   string `yaml:"prefix,omitempty"`    // key prefix in the bucket
	Endpoint string `yaml:"endpoint,omitempty"`  // custom endpoint (MinIO/LocalStack)
}

// BackendConfigPath returns the path to the backend config file.
func BackendConfigPath() (string, error) {
	dir, err := config.ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "backend.yaml"), nil
}

// LoadBackendConfig reads the backend config file.
// Returns a default local backend if the file doesn't exist.
func LoadBackendConfig() (*BackendConfig, error) {
	path, err := BackendConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &BackendConfig{Backend: BackendSpec{Type: "local"}}, nil
		}
		return nil, fmt.Errorf("reading backend config: %w", err)
	}

	var bc BackendConfig
	if err := yaml.Unmarshal(data, &bc); err != nil {
		return nil, fmt.Errorf("parsing backend config: %w", err)
	}

	if bc.Backend.Type == "" {
		bc.Backend.Type = "local"
	}

	return &bc, nil
}

// NewStoreFromConfig creates the appropriate Store based on backend configuration.
func NewStoreFromConfig(ctx context.Context) (Store, error) {
	bc, err := LoadBackendConfig()
	if err != nil {
		return nil, err
	}

	switch bc.Backend.Type {
	case "local", "":
		return NewDefaultLocalStore()
	case "s3":
		if bc.Backend.Bucket == "" {
			return nil, fmt.Errorf("S3 backend requires 'bucket' to be set in %s", "backend.yaml")
		}
		return NewS3Store(ctx, S3StoreOptions{
			Bucket:   bc.Backend.Bucket,
			Region:   bc.Backend.Region,
			Prefix:   bc.Backend.Prefix,
			Endpoint: bc.Backend.Endpoint,
		})
	default:
		return nil, fmt.Errorf("unknown backend type %q (supported: local, s3)", bc.Backend.Type)
	}
}
