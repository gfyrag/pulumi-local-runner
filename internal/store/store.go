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

	// StackFilePath returns the filesystem path to a stack's definition file.
	// For remote stores, this downloads to a temp file and returns that path.
	// The caller should call SaveStackFile after editing.
	StackFilePath(appName, stackName string) (string, error)

	// ReadBaseConfig reads a named base config template.
	// Returns nil, nil if it does not exist.
	ReadBaseConfig(name string) ([]byte, error)

	// WriteBaseConfig writes a named base config template.
	WriteBaseConfig(name string, data []byte) error

	// ListBases returns the names of all stored bases.
	ListBases() ([]string, error)

	// DeleteBaseConfig removes a named base.
	DeleteBaseConfig(name string) error
}
