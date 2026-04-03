package store

import "github.com/gfyrag/plr/internal/config"

// Store abstracts the persistence of PLR configuration.
// Implementations may store data locally (filesystem) or remotely (S3, etc.).
type Store interface {
	// LoadConfig reads the main PLR configuration.
	LoadConfig() (*config.Config, error)

	// SaveConfig writes the main PLR configuration.
	SaveConfig(cfg *config.Config) error

	// ReadStackConfig reads a Pulumi stack config file.
	// Returns nil, nil if the config does not exist.
	ReadStackConfig(appName, stackName string) ([]byte, error)

	// WriteStackConfig writes a Pulumi stack config file.
	WriteStackConfig(appName, stackName string, data []byte) error
}
