package git

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// mergeConfigs deep-merges base configs (in order) with the stack's own config on top.
// The config: map is deep-merged; encryptionsalt comes from the stack only.
// Returns nil if all inputs are nil.
func mergeConfigs(bases [][]byte, stackData []byte) ([]byte, error) {
	merged := make(map[string]any)

	for i, base := range bases {
		var m map[string]any
		if err := yaml.Unmarshal(base, &m); err != nil {
			return nil, fmt.Errorf("parsing base %d: %w", i, err)
		}
		if cfg, ok := m["config"]; ok {
			if cfgMap, ok := cfg.(map[string]any); ok {
				mergedCfg, _ := merged["config"].(map[string]any)
				if mergedCfg == nil {
					mergedCfg = make(map[string]any)
				}
				deepMerge(mergedCfg, cfgMap)
				merged["config"] = mergedCfg
			}
		}
	}

	if stackData != nil {
		var m map[string]any
		if err := yaml.Unmarshal(stackData, &m); err != nil {
			return nil, fmt.Errorf("parsing stack config: %w", err)
		}
		if cfg, ok := m["config"]; ok {
			if cfgMap, ok := cfg.(map[string]any); ok {
				mergedCfg, _ := merged["config"].(map[string]any)
				if mergedCfg == nil {
					mergedCfg = make(map[string]any)
				}
				deepMerge(mergedCfg, cfgMap)
				merged["config"] = mergedCfg
			}
		}
		// encryptionsalt comes from stack only
		if salt, ok := m["encryptionsalt"]; ok {
			merged["encryptionsalt"] = salt
		}
	}

	if len(merged) == 0 {
		return nil, nil
	}

	return yaml.Marshal(merged)
}

// SecretValue represents a plaintext secret extracted from the config.
type SecretValue struct {
	Key   string
	Value string
}

// ExtractSecretValues removes keys marked as secret from the config YAML
// and returns them as plaintext key-value pairs. The YAML is re-marshaled without them.
func ExtractSecretValues(data []byte, secretKeys map[string]bool) ([]byte, []SecretValue, error) {
	if data == nil || len(secretKeys) == 0 {
		return data, nil, nil
	}

	var m map[string]any
	if err := yaml.Unmarshal(data, &m); err != nil {
		return data, nil, err
	}

	cfg, ok := m["config"].(map[string]any)
	if !ok {
		return data, nil, nil
	}

	var secrets []SecretValue
	for k, v := range cfg {
		if !secretKeys[k] {
			continue
		}
		// Only extract string values (not already-encrypted {secure:...} objects)
		switch val := v.(type) {
		case string:
			secrets = append(secrets, SecretValue{Key: k, Value: val})
			delete(cfg, k)
		}
	}

	if len(secrets) == 0 {
		return data, nil, nil
	}

	out, err := yaml.Marshal(m)
	if err != nil {
		return data, nil, err
	}
	return out, secrets, nil
}

// deepMerge merges src into dst. For map values, it recurses. For anything else, src wins.
func deepMerge(dst, src map[string]any) {
	for k, srcVal := range src {
		dstVal, exists := dst[k]
		if !exists {
			dst[k] = srcVal
			continue
		}

		dstMap, dstOk := dstVal.(map[string]any)
		srcMap, srcOk := srcVal.(map[string]any)
		if dstOk && srcOk {
			deepMerge(dstMap, srcMap)
		} else {
			dst[k] = srcVal
		}
	}
}
