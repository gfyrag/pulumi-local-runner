package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
//	<configDir>/apps/<app>/app.yaml
//	<configDir>/apps/<app>/<stack>.yaml
//	<configDir>/stacks/<app>/Pulumi.<stack>.yaml
//	<configDir>/bases/<name>.yaml
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

func (s *LocalStore) appsDir() string {
	return filepath.Join(s.configDir, "apps")
}

func (s *LocalStore) stackFilePath(appName, stackName string) string {
	return filepath.Join(s.appsDir(), appName, stackName+".yaml")
}

func (s *LocalStore) LoadConfig() (*config.Config, error) {
	appsDir := s.appsDir()
	appDirs, err := os.ReadDir(appsDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &config.Config{}, nil
		}
		return nil, fmt.Errorf("reading apps directory: %w", err)
	}

	var cfg config.Config
	for _, d := range appDirs {
		if !d.IsDir() {
			continue
		}
		appName := d.Name()
		appDir := filepath.Join(appsDir, appName)

		// Read app.yaml
		appData, err := os.ReadFile(filepath.Join(appDir, "app.yaml"))
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, fmt.Errorf("reading app %q: %w", appName, err)
		}

		var af config.AppFile
		if err := yaml.Unmarshal(appData, &af); err != nil {
			return nil, fmt.Errorf("parsing app %q: %w", appName, err)
		}

		app := config.App{
			Name: appName,
			Repo: af.Repo,
			Path: af.Path,
		}
		if app.Path == "" {
			app.Path = "."
		}

		// Read stack files (everything except app.yaml)
		entries, err := os.ReadDir(appDir)
		if err != nil {
			return nil, fmt.Errorf("reading stacks for app %q: %w", appName, err)
		}
		for _, e := range entries {
			if e.IsDir() || e.Name() == "app.yaml" || !strings.HasSuffix(e.Name(), ".yaml") {
				continue
			}
			stackName := strings.TrimSuffix(e.Name(), ".yaml")

			stackData, err := os.ReadFile(filepath.Join(appDir, e.Name()))
			if err != nil {
				return nil, fmt.Errorf("reading stack %s/%s: %w", appName, stackName, err)
			}

			var sf config.StackFile
			if err := yaml.Unmarshal(stackData, &sf); err != nil {
				return nil, fmt.Errorf("parsing stack %s/%s: %w", appName, stackName, err)
			}

			app.Stacks = append(app.Stacks, config.Stack{
				Name:      stackName,
				Branch:    sf.Branch,
				Ref:       sf.Ref,
				DependsOn: sf.DependsOn,
				Org:       sf.Org,
				Project:   sf.Project,
				Bases:     sf.Bases,
			})
		}

		cfg.Apps = append(cfg.Apps, app)
	}

	return &cfg, nil
}

func (s *LocalStore) SaveConfig(cfg *config.Config) error {
	appsDir := s.appsDir()

	// Track which app dirs should exist
	wantApps := make(map[string]bool)
	for _, app := range cfg.Apps {
		wantApps[app.Name] = true
		appDir := filepath.Join(appsDir, app.Name)
		if err := os.MkdirAll(appDir, 0o755); err != nil {
			return fmt.Errorf("creating app directory %q: %w", app.Name, err)
		}

		// Write app.yaml
		af := config.AppFile{Repo: app.Repo, Path: app.Path}
		data, err := yaml.Marshal(af)
		if err != nil {
			return fmt.Errorf("marshaling app %q: %w", app.Name, err)
		}
		if err := os.WriteFile(filepath.Join(appDir, "app.yaml"), data, 0o644); err != nil {
			return err
		}

		// Write stack files and track which ones should exist
		wantStacks := make(map[string]bool)
		for _, stack := range app.Stacks {
			wantStacks[stack.Name+".yaml"] = true
			stackPath := filepath.Join(appDir, stack.Name+".yaml")

			// Read existing file to preserve config
			var existing config.StackFile
			if existingData, readErr := os.ReadFile(stackPath); readErr == nil {
				yaml.Unmarshal(existingData, &existing)
			}

			sf := config.StackFile{
				Branch:    stack.Branch,
				Ref:       stack.Ref,
				DependsOn: stack.DependsOn,
				Org:       stack.Org,
				Project:   stack.Project,
				Bases:     stack.Bases,
				Config:    existing.Config,
			}
			data, err := yaml.Marshal(sf)
			if err != nil {
				return fmt.Errorf("marshaling stack %s/%s: %w", app.Name, stack.Name, err)
			}
			if err := os.WriteFile(stackPath, data, 0o644); err != nil {
				return err
			}
		}

		// Remove stack files that no longer exist
		entries, _ := os.ReadDir(appDir)
		for _, e := range entries {
			if e.Name() == "app.yaml" || e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
				continue
			}
			if !wantStacks[e.Name()] {
				os.Remove(filepath.Join(appDir, e.Name()))
			}
		}
	}

	// Remove app dirs that no longer exist
	if dirs, err := os.ReadDir(appsDir); err == nil {
		for _, d := range dirs {
			if d.IsDir() && !wantApps[d.Name()] {
				os.RemoveAll(filepath.Join(appsDir, d.Name()))
			}
		}
	}

	return nil
}

func (s *LocalStore) ReadStackConfig(appName, stackName string) ([]byte, error) {
	data, err := os.ReadFile(s.stackFilePath(appName, stackName))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	var sf config.StackFile
	if err := yaml.Unmarshal(data, &sf); err != nil {
		return nil, fmt.Errorf("parsing stack file: %w", err)
	}

	if sf.Config == nil {
		return nil, nil
	}

	pulumiCfg := map[string]any{"config": sf.Config}
	return yaml.Marshal(pulumiCfg)
}

func (s *LocalStore) WriteStackConfig(appName, stackName string, data []byte) error {
	path := s.stackFilePath(appName, stackName)

	// Parse the incoming Pulumi config
	var incoming map[string]any
	if err := yaml.Unmarshal(data, &incoming); err != nil {
		return fmt.Errorf("parsing incoming config: %w", err)
	}

	// Read existing stack file to preserve definition fields
	existing, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	var sf config.StackFile
	if existing != nil {
		if err := yaml.Unmarshal(existing, &sf); err != nil {
			return fmt.Errorf("parsing existing stack file: %w", err)
		}
	}

	// Update config only (encryptionsalt is managed globally)
	if cfgVal, ok := incoming["config"]; ok {
		if cfgMap, ok := cfgVal.(map[string]any); ok {
			sf.Config = cfgMap
		}
	} else {
		sf.Config = nil
	}

	out, err := yaml.Marshal(sf)
	if err != nil {
		return fmt.Errorf("marshaling stack file: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o644)
}

func (s *LocalStore) StackFilePath(appName, stackName string) (string, error) {
	path := filepath.Join(s.appsDir(), appName, stackName+".yaml")
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("stack %s/%s not found", appName, stackName)
		}
		return "", err
	}
	return path, nil
}

func (s *LocalStore) baseConfigPath(name string) string {
	return filepath.Join(s.configDir, "bases", fmt.Sprintf("%s.yaml", name))
}

func (s *LocalStore) ReadBaseConfig(name string) ([]byte, error) {
	data, err := os.ReadFile(s.baseConfigPath(name))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	return data, nil
}

func (s *LocalStore) WriteBaseConfig(name string, data []byte) error {
	path := s.baseConfigPath(name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func (s *LocalStore) ListBases() ([]string, error) {
	dir := filepath.Join(s.configDir, "bases")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		names = append(names, strings.TrimSuffix(e.Name(), ".yaml"))
	}
	return names, nil
}

func (s *LocalStore) DeleteBaseConfig(name string) error {
	err := os.Remove(s.baseConfigPath(name))
	if err != nil && errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("base %q not found", name)
	}
	return err
}

func (s *LocalStore) encryptionSaltPath() string {
	return filepath.Join(s.configDir, "encryptionsalt")
}

func (s *LocalStore) ReadEncryptionSalt() (string, error) {
	data, err := os.ReadFile(s.encryptionSaltPath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func (s *LocalStore) WriteEncryptionSalt(salt string) error {
	if err := os.MkdirAll(s.configDir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(s.encryptionSaltPath(), []byte(salt+"\n"), 0o644)
}
