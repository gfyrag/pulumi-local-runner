package pulumi

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

// ConfigSchemaEntry describes a single config key from Pulumi.yaml.
type ConfigSchemaEntry struct {
	Key         string
	Type        string
	Description string
	Default     any
	Secret      bool
	Required    bool
	Properties  []ConfigSchemaEntry
}

// HasDefault returns true if a default value is defined.
func (e ConfigSchemaEntry) HasDefault() bool {
	return e.Default != nil
}

// ConfigSchema holds the parsed config schema from a Pulumi.yaml.
type ConfigSchema struct {
	ProjectName string
	Entries     []ConfigSchemaEntry
}

// Required returns entries explicitly marked as required.
func (s ConfigSchema) Required() []ConfigSchemaEntry {
	var result []ConfigSchemaEntry
	for _, e := range s.Entries {
		if e.Required {
			result = append(result, e)
		}
	}
	return result
}

// pulumiYAML is the structure of Pulumi.yaml for parsing.
type pulumiYAML struct {
	Name      string         `yaml:"name"`
	PLRConfig map[string]any `yaml:"x-plr-config"`
}

// LoadConfigSchema parses the config section from a Pulumi.yaml in the given workdir.
// Returns nil if no config schema is defined.
func LoadConfigSchema(workDir string) (*ConfigSchema, error) {
	data, err := os.ReadFile(filepath.Join(workDir, "Pulumi.yaml"))
	if err != nil {
		return nil, fmt.Errorf("reading Pulumi.yaml: %w", err)
	}

	var proj pulumiYAML
	if err := yaml.Unmarshal(data, &proj); err != nil {
		return nil, fmt.Errorf("parsing Pulumi.yaml: %w", err)
	}

	if len(proj.PLRConfig) == 0 {
		return nil, nil
	}

	schema := &ConfigSchema{ProjectName: proj.Name}

	schema.Entries = parseEntries(proj.PLRConfig)
	return schema, nil
}

// parseEntries recursively parses a map of config entries.
func parseEntries(m map[string]any) []ConfigSchemaEntry {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var entries []ConfigSchemaEntry
	for _, key := range keys {
		entries = append(entries, parseEntry(key, m[key]))
	}
	return entries
}

func parseEntry(key string, v any) ConfigSchemaEntry {
	entry := ConfigSchemaEntry{Key: key}

	switch val := v.(type) {
	case string:
		entry.Type = val
	case map[string]any:
		if t, ok := val["type"].(string); ok {
			entry.Type = t
		}
		if d, ok := val["description"].(string); ok {
			entry.Description = d
		}
		if def, ok := val["default"]; ok {
			entry.Default = def
		}
		if sec, ok := val["secret"].(bool); ok {
			entry.Secret = sec
		}
		if req, ok := val["required"].(bool); ok {
			entry.Required = req
		}
		if props, ok := val["properties"].(map[string]any); ok {
			entry.Properties = parseEntries(props)
		}
	}

	if entry.Type == "" {
		entry.Type = "string"
	}

	return entry
}

// ValidationResult holds the results of config validation.
type ValidationResult struct {
	Missing []string
	Unknown []string
}

// SecretKeys returns all keys marked as secret in the schema.
func (s ConfigSchema) SecretKeys() map[string]bool {
	keys := make(map[string]bool)
	for _, e := range s.Entries {
		if e.Secret {
			keys[e.Key] = true
			keys[s.ProjectName+":"+e.Key] = true
		}
	}
	return keys
}

// ValidateConfig checks config against the schema.
// Returns missing required keys and unknown keys not in the schema.
func ValidateConfig(schema *ConfigSchema, currentConfig map[string]any) ValidationResult {
	var result ValidationResult
	if schema == nil {
		return result
	}

	// Check missing required keys
	for _, entry := range schema.Required() {
		namespacedKey := schema.ProjectName + ":" + entry.Key
		if _, ok := currentConfig[entry.Key]; ok {
			continue
		}
		if _, ok := currentConfig[namespacedKey]; ok {
			continue
		}
		result.Missing = append(result.Missing, entry.Key)
	}

	// Check unknown keys
	knownKeys := make(map[string]bool)
	for _, entry := range schema.Entries {
		knownKeys[entry.Key] = true
		knownKeys[schema.ProjectName+":"+entry.Key] = true
	}
	for key := range currentConfig {
		if !knownKeys[key] {
			result.Unknown = append(result.Unknown, key)
		}
	}

	return result
}
