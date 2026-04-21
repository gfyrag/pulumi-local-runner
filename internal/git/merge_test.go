package git

import (
	"fmt"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestMergeConfigs(t *testing.T) {
	tests := []struct {
		name      string
		bases     []string
		stack     string
		wantKeys  map[string]string // expected config key=value
		wantSalt  string
		wantNil   bool
	}{
		{
			name:    "no bases no stack",
			wantNil: true,
		},
		{
			name:  "stack only",
			stack: "config:\n  key1: val1\nencryptionsalt: v1:abc",
			wantKeys: map[string]string{
				"key1": "val1",
			},
			wantSalt: "v1:abc",
		},
		{
			name:  "single base with stack overlay",
			bases: []string{"config:\n  key1: base1\n  key2: base2"},
			stack: "config:\n  key1: stack1\nencryptionsalt: v1:xyz",
			wantKeys: map[string]string{
				"key1": "stack1", // stack wins
				"key2": "base2", // from base
			},
			wantSalt: "v1:xyz",
		},
		{
			name: "multiple bases layered",
			bases: []string{
				"config:\n  key1: base1\n  key2: base1",
				"config:\n  key2: base2\n  key3: base2",
			},
			stack: "config:\n  key3: stack\nencryptionsalt: v1:salt",
			wantKeys: map[string]string{
				"key1": "base1", // from first base
				"key2": "base2", // second base wins
				"key3": "stack", // stack wins
			},
			wantSalt: "v1:salt",
		},
		{
			name:  "base only no stack",
			bases: []string{"config:\n  key1: val1"},
			wantKeys: map[string]string{
				"key1": "val1",
			},
		},
		{
			name:  "encryptionsalt from stack only",
			bases: []string{"config:\n  key1: val1\nencryptionsalt: v1:base"},
			stack: "config:\n  key2: val2\nencryptionsalt: v1:stack",
			wantKeys: map[string]string{
				"key1": "val1",
				"key2": "val2",
			},
			wantSalt: "v1:stack",
		},
		{
			name:  "deep nested map merge",
			bases: []string{"config:\n  nested:\n    a: 1\n    b: 2"},
			stack: "config:\n  nested:\n    b: 3\n    c: 4",
			wantKeys: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var bases [][]byte
			for _, b := range tt.bases {
				bases = append(bases, []byte(b))
			}
			var stackData []byte
			if tt.stack != "" {
				stackData = []byte(tt.stack)
			}

			result, err := mergeConfigs(bases, stackData)
			if err != nil {
				t.Fatalf("mergeConfigs() error = %v", err)
			}
			if tt.wantNil {
				if result != nil {
					t.Fatalf("expected nil, got %s", result)
				}
				return
			}

			var m map[string]any
			if err := yaml.Unmarshal(result, &m); err != nil {
				t.Fatalf("failed to parse result: %v", err)
			}

			cfg, _ := m["config"].(map[string]any)
			for k, want := range tt.wantKeys {
				got, ok := cfg[k]
				if !ok {
					t.Errorf("missing key %q", k)
					continue
				}
				if fmt.Sprint(got) != want {
					t.Errorf("config[%q] = %v, want %v", k, got, want)
				}
			}

			if tt.wantSalt != "" {
				if salt, _ := m["encryptionsalt"].(string); salt != tt.wantSalt {
					t.Errorf("encryptionsalt = %q, want %q", salt, tt.wantSalt)
				}
			}
		})
	}
}

func TestMergeConfigsDeepNested(t *testing.T) {
	base := []byte("config:\n  nested:\n    a: 1\n    b: 2")
	stack := []byte("config:\n  nested:\n    b: 3\n    c: 4")

	result, err := mergeConfigs([][]byte{base}, stack)
	if err != nil {
		t.Fatalf("mergeConfigs() error = %v", err)
	}

	var m map[string]any
	if err := yaml.Unmarshal(result, &m); err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	cfg := m["config"].(map[string]any)
	nested := cfg["nested"].(map[string]any)

	if nested["a"] != 1 {
		t.Errorf("nested.a = %v, want 1", nested["a"])
	}
	if nested["b"] != 3 {
		t.Errorf("nested.b = %v, want 3", nested["b"])
	}
	if nested["c"] != 4 {
		t.Errorf("nested.c = %v, want 4", nested["c"])
	}
}
