package main

import (
	"os/exec"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func requireHelm(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("helm"); err != nil {
		t.Skip("helm not found in PATH, skipping Helm template test")
	}
}

// helmTemplate runs helm template with the given values and returns the rendered YAML.
func helmTemplate(t *testing.T, setValues ...string) string {
	t.Helper()
	requireHelm(t)
	args := []string{"template", "test-release", "../../helm/operator"}
	for _, v := range setValues {
		args = append(args, "--set", v)
	}
	cmd := exec.Command("helm", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("helm template failed: %v\n%s", err, out)
	}
	return string(out)
}

// extractDeploymentEnvVars parses multi-doc YAML, finds the Deployment, and returns
// the env vars of the first container as a map[name]value.
func extractDeploymentEnvVars(t *testing.T, rendered string) map[string]string {
	t.Helper()
	envs := make(map[string]string)

	docs := strings.Split(rendered, "---")
	for _, doc := range docs {
		doc = strings.TrimSpace(doc)
		if doc == "" {
			continue
		}
		var obj map[string]any
		if err := yaml.Unmarshal([]byte(doc), &obj); err != nil {
			continue
		}
		if obj["kind"] != "Deployment" {
			continue
		}

		spec := obj["spec"].(map[string]any)
		template := spec["template"].(map[string]any)
		podSpec := template["spec"].(map[string]any)
		containers := podSpec["containers"].([]any)
		container := containers[0].(map[string]any)

		envList, ok := container["env"].([]any)
		if !ok {
			return envs
		}
		for _, e := range envList {
			env := e.(map[string]any)
			name, _ := env["name"].(string)
			value, _ := env["value"].(string)
			envs[name] = value
		}
		return envs
	}

	t.Fatal("no Deployment found in rendered output")
	return nil
}

func TestHelmTemplate_DefaultLedgerImage(t *testing.T) {
	rendered := helmTemplate(t)
	envs := extractDeploymentEnvVars(t, rendered)

	repo := envs["LEDGER_IMAGE_REPOSITORY"]
	if repo != "ghcr.io/formancehq/ledger" {
		t.Errorf("expected LEDGER_IMAGE_REPOSITORY=ghcr.io/formancehq/ledger, got %q", repo)
	}

	tag := envs["LEDGER_IMAGE_TAG"]
	if tag != "latest" {
		t.Errorf("expected LEDGER_IMAGE_TAG=latest, got %q", tag)
	}
}

func TestHelmTemplate_CustomRegistry(t *testing.T) {
	rendered := helmTemplate(t, "ledgerImage.registry=registry.example.com")
	envs := extractDeploymentEnvVars(t, rendered)

	repo := envs["LEDGER_IMAGE_REPOSITORY"]
	if repo != "registry.example.com/formancehq/ledger" {
		t.Errorf("expected LEDGER_IMAGE_REPOSITORY=registry.example.com/formancehq/ledger, got %q", repo)
	}
}

func TestHelmTemplate_CustomName(t *testing.T) {
	rendered := helmTemplate(t, "ledgerImage.name=myorg/custom-ledger")
	envs := extractDeploymentEnvVars(t, rendered)

	repo := envs["LEDGER_IMAGE_REPOSITORY"]
	if repo != "ghcr.io/myorg/custom-ledger" {
		t.Errorf("expected LEDGER_IMAGE_REPOSITORY=ghcr.io/myorg/custom-ledger, got %q", repo)
	}
}

func TestHelmTemplate_CustomRegistryAndName(t *testing.T) {
	rendered := helmTemplate(t,
		"ledgerImage.registry=my-registry.io",
		"ledgerImage.name=acme/ledger-custom",
		"ledgerImage.tag=v2.0.0",
	)
	envs := extractDeploymentEnvVars(t, rendered)

	repo := envs["LEDGER_IMAGE_REPOSITORY"]
	if repo != "my-registry.io/acme/ledger-custom" {
		t.Errorf("expected LEDGER_IMAGE_REPOSITORY=my-registry.io/acme/ledger-custom, got %q", repo)
	}

	tag := envs["LEDGER_IMAGE_TAG"]
	if tag != "v2.0.0" {
		t.Errorf("expected LEDGER_IMAGE_TAG=v2.0.0, got %q", tag)
	}
}

func TestHelmTemplate_CustomTag(t *testing.T) {
	rendered := helmTemplate(t, "ledgerImage.tag=sha-abc123")
	envs := extractDeploymentEnvVars(t, rendered)

	tag := envs["LEDGER_IMAGE_TAG"]
	if tag != "sha-abc123" {
		t.Errorf("expected LEDGER_IMAGE_TAG=sha-abc123, got %q", tag)
	}
}
