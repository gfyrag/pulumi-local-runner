// Package shared provides common helpers for devenv Pulumi sub-projects.
package shared

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
	"gopkg.in/yaml.v3"
)

// GetConfigObject reads a config object from Pulumi config. If the object
// contains a "file" key, reads the YAML file instead (path relative to basePath).
func GetConfigObject(cfg *config.Config, key string, basePath string) (map[string]any, error) {
	var configObj map[string]any
	if err := cfg.GetObject(key, &configObj); err != nil {
		return nil, fmt.Errorf("failed to get config object %s: %w", key, err)
	}

	if filePath, ok := configObj["file"].(string); ok {
		fullPath := filepath.Join(basePath, filePath)
		data, err := os.ReadFile(fullPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read values file %s: %w", fullPath, err)
		}

		var result map[string]any
		if err := yaml.Unmarshal(data, &result); err != nil {
			return nil, fmt.Errorf("failed to parse YAML file %s: %w", fullPath, err)
		}

		return result, nil
	}

	return configObj, nil
}

// K8sSetup holds the common Kubernetes provider and namespace setup.
type K8sSetup struct {
	Provider      pulumi.ProviderResource
	NamespaceName pulumi.StringOutput
}

// NewK8sSetup creates a Kubernetes provider and resolves the target namespace
// from config. The namespace is NOT managed by Pulumi — it must already exist.
func NewK8sSetup(ctx *pulumi.Context, cfg *config.Config) (*K8sSetup, error) {
	kubeContext := cfg.Require("k8s-context")

	namespaceName := cfg.Get("namespace")
	if namespaceName == "" {
		namespaceName = ctx.Stack()
	}

	k8sProvider, err := NewK8sProvider(ctx, kubeContext)
	if err != nil {
		return nil, err
	}

	return &K8sSetup{
		Provider:      k8sProvider,
		NamespaceName: pulumi.String(namespaceName).ToStringOutput(),
	}, nil
}

// NewK8sProvider creates a Kubernetes provider for the given context.
func NewK8sProvider(ctx *pulumi.Context, kubeContext string) (pulumi.ProviderResource, error) {
	return newK8sProviderInternal(ctx, kubeContext)
}

// GetConfigMap reads an optional object config and returns it as a pulumi.Map.
func GetConfigMap(cfg *config.Config, key string) pulumi.Map {
	var obj map[string]any
	if err := cfg.GetObject(key, &obj); err != nil || obj == nil {
		return pulumi.Map{}
	}
	return pulumi.ToMap(obj)
}

// GetConfigArray reads an optional array config and returns it as a pulumi.Array.
func GetConfigArray(cfg *config.Config, key string) pulumi.Array {
	var arr []map[string]any
	if err := cfg.GetObject(key, &arr); err != nil || arr == nil {
		return pulumi.Array{}
	}
	result := make(pulumi.Array, len(arr))
	for i, v := range arr {
		result[i] = pulumi.ToMap(v)
	}
	return result
}

// GetConfigStringArray reads an optional string array config and returns it as a pulumi.Array.
func GetConfigStringArray(cfg *config.Config, key string) pulumi.Array {
	var arr []string
	if err := cfg.GetObject(key, &arr); err != nil || len(arr) == 0 {
		return pulumi.Array{}
	}
	result := make(pulumi.Array, len(arr))
	for i, v := range arr {
		result[i] = pulumi.String(v)
	}
	return result
}

// GetImagePullSecrets reads an optional list of image pull secret references from config.
func GetImagePullSecrets(cfg *config.Config) pulumi.Array {
	var secrets []map[string]any
	if err := cfg.GetObject("image-pull-secrets", &secrets); err != nil || len(secrets) == 0 {
		return pulumi.Array{}
	}
	var result pulumi.Array
	for _, s := range secrets {
		if name, ok := s["name"].(string); ok && name != "" {
			result = append(result, pulumi.Map{
				"name": pulumi.String(name),
			})
		}
	}
	return result
}

// IngressValues builds the ingress Helm values from config.
func IngressValues(cfg *config.Config, key string) pulumi.Map {
	var raw struct {
		Annotations map[string]string `json:"annotations"`
		ClassName   string            `json:"className"`
		TLS         []struct {
			SecretName string   `json:"secretName"`
			Hosts      []string `json:"hosts"`
		} `json:"tls"`
	}
	if err := cfg.GetObject(key, &raw); err != nil {
		return pulumi.Map{}
	}
	ingress := pulumi.Map{}
	if len(raw.Annotations) > 0 {
		annotations := pulumi.Map{}
		for k, v := range raw.Annotations {
			annotations[k] = pulumi.String(v)
		}
		ingress["annotations"] = annotations
	}
	if raw.ClassName != "" {
		ingress["className"] = pulumi.String(raw.ClassName)
	}
	if len(raw.TLS) > 0 {
		tls := make(pulumi.Array, len(raw.TLS))
		for i, t := range raw.TLS {
			hosts := make(pulumi.Array, len(t.Hosts))
			for j, h := range t.Hosts {
				hosts[j] = pulumi.String(h)
			}
			tls[i] = pulumi.Map{
				"secretName": pulumi.String(t.SecretName),
				"hosts":      hosts,
			}
		}
		ingress["tls"] = tls
	}
	return ingress
}

