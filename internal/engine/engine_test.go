package engine

import (
	"testing"

	"github.com/gfyrag/plr/internal/config"
)

func makeConfig() *config.Config {
	return &config.Config{
		Apps: []config.App{
			{
				Name: "networking",
				Repo: "git@github.com:org/networking.git",
				Stacks: []config.Stack{
					{Name: "dev", Branch: "main"},
					{Name: "prod", Ref: "v1.0.0"},
				},
			},
			{
				Name: "kubernetes",
				Repo: "git@github.com:org/kubernetes.git",
				Stacks: []config.Stack{
					{Name: "dev", Branch: "main", DependsOn: []string{"networking/dev"}},
					{Name: "prod", Branch: "release", DependsOn: []string{"networking/prod"}},
				},
			},
		},
	}
}

func TestResolveTargetsAll(t *testing.T) {
	cfg := makeConfig()
	targets, err := ResolveTargets(cfg, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(targets) != 4 {
		t.Fatalf("expected 4 targets, got %d", len(targets))
	}
}

func TestResolveTargetsByApp(t *testing.T) {
	cfg := makeConfig()
	targets, err := ResolveTargets(cfg, []string{"networking"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(targets) != 2 {
		t.Fatalf("expected 2 targets, got %d", len(targets))
	}
	for _, tgt := range targets {
		if tgt.App.Name != "networking" {
			t.Errorf("expected app networking, got %s", tgt.App.Name)
		}
	}
}

func TestResolveTargetsByAppStack(t *testing.T) {
	cfg := makeConfig()
	targets, err := ResolveTargets(cfg, []string{"kubernetes/prod"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(targets))
	}
	if targets[0].Key() != "kubernetes/prod" {
		t.Errorf("got %s", targets[0].Key())
	}
}

func TestResolveTargetsUnknownApp(t *testing.T) {
	cfg := makeConfig()
	_, err := ResolveTargets(cfg, []string{"unknown"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveTargetsUnknownStack(t *testing.T) {
	cfg := makeConfig()
	_, err := ResolveTargets(cfg, []string{"networking/staging"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestTopoSortNoDeps(t *testing.T) {
	targets := []Target{
		{App: &config.App{Name: "a"}, Stack: &config.Stack{Name: "dev"}},
		{App: &config.App{Name: "b"}, Stack: &config.Stack{Name: "dev"}},
	}
	ordered, err := topoSort(targets)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ordered) != 2 {
		t.Fatalf("expected 2, got %d", len(ordered))
	}
}

func TestTopoSortWithDeps(t *testing.T) {
	targets := []Target{
		{App: &config.App{Name: "k8s"}, Stack: &config.Stack{Name: "dev", DependsOn: []string{"net/dev"}}},
		{App: &config.App{Name: "net"}, Stack: &config.Stack{Name: "dev"}},
	}
	ordered, err := topoSort(targets)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ordered[0].Key() != "net/dev" {
		t.Errorf("expected net/dev first, got %s", ordered[0].Key())
	}
	if ordered[1].Key() != "k8s/dev" {
		t.Errorf("expected k8s/dev second, got %s", ordered[1].Key())
	}
}

func TestTopoSortCircular(t *testing.T) {
	targets := []Target{
		{App: &config.App{Name: "a"}, Stack: &config.Stack{Name: "dev", DependsOn: []string{"b/dev"}}},
		{App: &config.App{Name: "b"}, Stack: &config.Stack{Name: "dev", DependsOn: []string{"a/dev"}}},
	}
	_, err := topoSort(targets)
	if err == nil {
		t.Fatal("expected circular dependency error")
	}
}

func TestTopoSortChain(t *testing.T) {
	targets := []Target{
		{App: &config.App{Name: "c"}, Stack: &config.Stack{Name: "dev", DependsOn: []string{"b/dev"}}},
		{App: &config.App{Name: "b"}, Stack: &config.Stack{Name: "dev", DependsOn: []string{"a/dev"}}},
		{App: &config.App{Name: "a"}, Stack: &config.Stack{Name: "dev"}},
	}
	ordered, err := topoSort(targets)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Must be a -> b -> c
	if ordered[0].Key() != "a/dev" || ordered[1].Key() != "b/dev" || ordered[2].Key() != "c/dev" {
		t.Errorf("wrong order: %s, %s, %s", ordered[0].Key(), ordered[1].Key(), ordered[2].Key())
	}
}

func TestTopoSortExternalDep(t *testing.T) {
	targets := []Target{
		{App: &config.App{Name: "k8s"}, Stack: &config.Stack{Name: "dev", DependsOn: []string{"net/dev"}}},
	}
	// net/dev is not in targets, should be skipped
	ordered, err := topoSort(targets)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ordered) != 1 {
		t.Fatalf("expected 1, got %d", len(ordered))
	}
}

func TestOperationString(t *testing.T) {
	tests := []struct {
		op   Operation
		want string
	}{
		{OpUp, "up"},
		{OpPreview, "preview"},
		{OpDestroy, "destroy"},
		{OpRefresh, "refresh"},
		{OpSync, "sync"},
	}
	for _, tt := range tests {
		if got := tt.op.String(); got != tt.want {
			t.Errorf("Operation(%d).String() = %q, want %q", tt.op, got, tt.want)
		}
	}
}

func TestTargetKey(t *testing.T) {
	tgt := Target{
		App:   &config.App{Name: "myapp"},
		Stack: &config.Stack{Name: "prod"},
	}
	if tgt.Key() != "myapp/prod" {
		t.Errorf("got %q", tgt.Key())
	}
}

func TestSetNestedValueSimple(t *testing.T) {
	m := map[string]any{}
	setNestedValue(m, "a.b.c", "val")
	c := m["a"].(map[string]any)["b"].(map[string]any)["c"]
	if c != "val" {
		t.Errorf("got %v", c)
	}
}

func TestSetNestedValueArrayIndex(t *testing.T) {
	m := map[string]any{}
	setNestedValue(m, "resources[0].manifest.spec.image.tag", "latest")

	arr := m["resources"].([]any)
	if len(arr) != 1 {
		t.Fatalf("expected 1 element, got %d", len(arr))
	}
	tag := arr[0].(map[string]any)["manifest"].(map[string]any)["spec"].(map[string]any)["image"].(map[string]any)["tag"]
	if tag != "latest" {
		t.Errorf("got %v", tag)
	}
}

func TestSetNestedValueArrayIndexPreservesExisting(t *testing.T) {
	m := map[string]any{
		"resources": []any{
			map[string]any{"name": "existing"},
		},
	}
	setNestedValue(m, "resources[0].image", "new")

	elem := m["resources"].([]any)[0].(map[string]any)
	if elem["name"] != "existing" {
		t.Errorf("existing value lost: %v", elem)
	}
	if elem["image"] != "new" {
		t.Errorf("new value not set: %v", elem)
	}
}

func TestSetNestedValueArrayGrows(t *testing.T) {
	m := map[string]any{}
	setNestedValue(m, "items[2].name", "third")

	arr := m["items"].([]any)
	if len(arr) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(arr))
	}
	if arr[0] != nil || arr[1] != nil {
		t.Errorf("expected nil padding, got %v, %v", arr[0], arr[1])
	}
	name := arr[2].(map[string]any)["name"]
	if name != "third" {
		t.Errorf("got %v", name)
	}
}

func TestSetNestedValueArrayAsLeaf(t *testing.T) {
	m := map[string]any{}
	setNestedValue(m, "tags[1]", "v2")

	arr := m["tags"].([]any)
	if len(arr) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(arr))
	}
	if arr[1] != "v2" {
		t.Errorf("got %v", arr[1])
	}
}
