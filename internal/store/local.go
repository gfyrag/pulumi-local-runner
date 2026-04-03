package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gfyrag/plr/internal/config"
	"gopkg.in/yaml.v3"
)

// LocalStore stores configuration on the local filesystem using XDG directories.
type LocalStore struct {
	configDir string
}

// NewLocalStore creates a store backed by the given directory.
// The directory structure is:
//
//	<configDir>/config.yaml
//	<configDir>/stacks/<app>/Pulumi.<stack>.yaml
func NewLocalStore(configDir string) *LocalStore {
	return &LocalStore{configDir: configDir}
}

// NewDefaultLocalStore creates a store using the XDG config directory.
func NewDefaultLocalStore() (*LocalStore, error) {
	dir, err := config.ConfigDir()
	if err != nil {
		return nil, err
	}
	return NewLocalStore(dir), nil
}

func (s *LocalStore) configPath() string {
	return filepath.Join(s.configDir, "config.yaml")
}

func (s *LocalStore) stackConfigPath(appName, stackName string) string {
	return filepath.Join(s.configDir, "stacks", appName, fmt.Sprintf("Pulumi.%s.yaml", stackName))
}

func (s *LocalStore) LoadConfig() (*config.Config, error) {
	data, err := os.ReadFile(s.configPath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &config.Config{}, nil
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var cfg config.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	for i := range cfg.Apps {
		if cfg.Apps[i].Path == "" {
			cfg.Apps[i].Path = "."
		}
	}

	return &cfg, nil
}

func (s *LocalStore) SaveConfig(cfg *config.Config) error {
	if err := os.MkdirAll(filepath.Dir(s.configPath()), 0o755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	return os.WriteFile(s.configPath(), data, 0o644)
}

func (s *LocalStore) ReadStackConfig(appName, stackName string) ([]byte, error) {
	data, err := os.ReadFile(s.stackConfigPath(appName, stackName))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	return data, nil
}

func (s *LocalStore) WriteStackConfig(appName, stackName string, data []byte) error {
	path := s.stackConfigPath(appName, stackName)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
