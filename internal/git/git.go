package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gfyrag/plr/internal/config"
	"github.com/gfyrag/plr/internal/store"
	"github.com/gfyrag/plr/internal/ui"
)

// IsLocalRepo returns true if the repo field points to a local directory.
func IsLocalRepo(repo string) bool {
	return strings.HasPrefix(repo, "/") || strings.HasPrefix(repo, "./") || strings.HasPrefix(repo, "../") || strings.HasPrefix(repo, "~")
}

// RepoDirForStack returns the repo directory, using stack override if set.
func RepoDirForStack(app *config.App, stack *config.Stack) (string, error) {
	repo := config.EffectiveRepo(app, stack)
	if IsLocalRepo(repo) {
		if strings.HasPrefix(repo, "~") {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", fmt.Errorf("could not determine home directory: %w", err)
			}
			repo = filepath.Join(home, repo[1:])
		}
		return filepath.Clean(repo), nil
	}
	cacheDir, err := config.CacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cacheDir, app.Name), nil
}

// RepoDir returns the local cache directory for the given app (no stack override).
func RepoDir(app *config.App) (string, error) {
	return RepoDirForStack(app, nil)
}

// WorkDirForStack returns the working directory, using stack overrides if set.
func WorkDirForStack(app *config.App, stack *config.Stack) (string, error) {
	repoDir, err := RepoDirForStack(app, stack)
	if err != nil {
		return "", err
	}
	return filepath.Join(repoDir, config.EffectivePath(app, stack)), nil
}

// WorkDir returns the working directory (repo + path) for the given app (no stack override).
func WorkDir(app *config.App) (string, error) {
	return WorkDirForStack(app, nil)
}

// Sync clones the repo if it doesn't exist, fetches updates, and checks out the given ref.
// For local repos, it skips clone/fetch and only checks out the requested ref (if any).
func Sync(s store.Store, app *config.App, stack *config.Stack) error {
	repo := config.EffectiveRepo(app, stack)
	repoDir, err := RepoDirForStack(app, stack)
	if err != nil {
		return err
	}

	if IsLocalRepo(repo) {
		if err := checkout(repoDir, stack); err != nil {
			return fmt.Errorf("checking out ref for stack %q: %w", stack.Name, err)
		}
	} else {
		if _, err := os.Stat(filepath.Join(repoDir, ".git")); os.IsNotExist(err) {
			ui.Info("Cloning %s...", repo)
			if err := runGit("clone", repo, repoDir); err != nil {
				return fmt.Errorf("cloning %s: %w", repo, err)
			}
		} else {
			ui.Step("Fetching updates...")
			if err := runGit("-C", repoDir, "fetch", "--all", "--tags", "--prune"); err != nil {
				return fmt.Errorf("fetching %s: %w", repo, err)
			}
		}

		if err := checkout(repoDir, stack); err != nil {
			return fmt.Errorf("checking out ref for stack %q: %w", stack.Name, err)
		}
	}

	return nil
}

func runGit(args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git %s: %w", args[0], err)
	}
	return nil
}

func checkout(repoDir string, stack *config.Stack) error {
	ref := stack.Branch
	if stack.Ref != "" {
		ref = stack.Ref
	}
	if ref == "" {
		return nil
	}

	target := ref
	if stack.Branch != "" {
		target = "origin/" + ref
	}

	// Reset any local modifications (e.g. Pulumi config files from previous runs)
	// so that checkout succeeds cleanly.
	if err := runGit("-C", repoDir, "checkout", "--", "."); err != nil {
		return fmt.Errorf("cleaning workdir: %w", err)
	}

	ui.Step("Checking out %s...", ref)
	return runGit("-C", repoDir, "checkout", "--detach", target)
}

// PrepareWorkDir writes the merged Pulumi config into the real workdir.
// Secret keys (from schema) are extracted from the YAML and returned separately
// to be set via the Automation API.
// Returns the workdir path, a cleanup function, and extracted secrets.
func PrepareWorkDir(s store.Store, app *config.App, stack *config.Stack, secretKeys map[string]bool) (string, func(), []SecretValue, error) {
	workDir, err := WorkDirForStack(app, stack)
	if err != nil {
		return "", nil, nil, err
	}

	configFile := filepath.Join(workDir, fmt.Sprintf("Pulumi.%s.yaml", stack.Name))

	data, err := BuildMergedConfig(s, app, stack)
	if err != nil {
		return "", nil, nil, err
	}

	// Extract plaintext secrets so they can be set via Automation API
	var secrets []SecretValue
	if data != nil {
		data, secrets, err = ExtractSecretValues(data, secretKeys)
		if err != nil {
			return "", nil, nil, fmt.Errorf("extracting secrets: %w", err)
		}
		if err := os.WriteFile(configFile, data, 0o644); err != nil {
			return "", nil, nil, err
		}
	}

	cleanup := func() { os.Remove(configFile) }
	return workDir, cleanup, secrets, nil
}

// BuildMergedConfig returns the merged config bytes for a stack (bases + stack overlay).
// Returns nil if no config exists and no bases are configured.
func BuildMergedConfig(s store.Store, app *config.App, stack *config.Stack) ([]byte, error) {
	stackData, err := s.ReadStackConfig(app.Name, stack.Name)
	if err != nil {
		return nil, err
	}

	if len(stack.Bases) == 0 {
		return stackData, nil
	}

	var bases [][]byte
	for _, baseName := range stack.Bases {
		baseData, err := s.ReadBaseConfig(baseName)
		if err != nil {
			return nil, fmt.Errorf("reading base %q: %w", baseName, err)
		}
		if baseData == nil {
			return nil, fmt.Errorf("base %q not found", baseName)
		}
		bases = append(bases, baseData)
	}

	return mergeConfigs(bases, stackData)
}

// SaveStackConfig copies the stack config from the workdir back to the store.
func SaveStackConfig(s store.Store, app *config.App, stack *config.Stack) error {
	workDir, err := WorkDir(app)
	if err != nil {
		return err
	}

	src := filepath.Join(workDir, fmt.Sprintf("Pulumi.%s.yaml", stack.Name))
	data, err := os.ReadFile(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	return s.WriteStackConfig(app.Name, stack.Name, data)
}
