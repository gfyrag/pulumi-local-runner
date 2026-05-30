package main

import (
	"fmt"
	"os"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
	"gopkg.in/yaml.v3"
)

// loadSpec reads the LedgerService spec from Pulumi config.
// It supports two modes:
//   - "spec-file": path to a YAML file containing the full spec
//   - "spec": inline YAML object in Pulumi config
func loadSpec(cfg *config.Config) (map[string]any, error) {
	if specFile := cfg.Get("spec-file"); specFile != "" {
		data, err := os.ReadFile(specFile)
		if err != nil {
			return nil, fmt.Errorf("reading spec file %s: %w", specFile, err)
		}
		var spec map[string]any
		if err := yaml.Unmarshal(data, &spec); err != nil {
			return nil, fmt.Errorf("parsing spec file %s: %w", specFile, err)
		}
		return spec, nil
	}

	var spec map[string]any
	if err := cfg.GetObject("spec", &spec); err == nil && spec != nil {
		return spec, nil
	}

	return nil, fmt.Errorf("either 'spec' or 'spec-file' must be set in config")
}

// loadBackup reads the optional LedgerBackup config and builds the spec.
// Returns nil if no backup config is set.
func loadBackup(cfg *config.Config, serviceRef string) (map[string]any, error) {
	var backup map[string]any
	if err := cfg.GetObject("backup", &backup); err != nil || backup == nil {
		return nil, nil
	}

	spec := map[string]any{
		"serviceRef": serviceRef,
	}

	if dest, ok := backup["destination"]; ok {
		spec["destination"] = dest
	}
	if schedule, ok := backup["schedule"]; ok {
		spec["schedule"] = schedule
	}

	return spec, nil
}
