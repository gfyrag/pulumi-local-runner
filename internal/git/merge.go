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

// injectSalt overrides the encryptionsalt in a Pulumi config YAML.
func injectSalt(data []byte, salt string) ([]byte, error) {
	if data == nil {
		m := map[string]any{"encryptionsalt": salt}
		return yaml.Marshal(m)
	}
	var m map[string]any
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing config for salt injection: %w", err)
	}
	m["encryptionsalt"] = salt
	return yaml.Marshal(m)
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
