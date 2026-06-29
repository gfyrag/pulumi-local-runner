package main

import (
	"fmt"

	v1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/core/v1"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"

	"github.com/gfyrag/pulumi-local-runner/apps/shared"
)

// k8sSetup holds the common Kubernetes provider and namespace setup.
type k8sSetup struct {
	Provider  pulumi.ProviderResource
	Namespace *v1.Namespace
}

// newK8sSetup creates a Kubernetes provider and namespace from config.
func newK8sSetup(ctx *pulumi.Context, cfg *config.Config) (*k8sSetup, error) {
	kubeContext := cfg.Require("k8s-context")

	namespaceName := cfg.Get("namespace")
	if namespaceName == "" {
		namespaceName = ctx.Stack()
	}

	k8sProvider, err := shared.NewK8sProvider(ctx, kubeContext)
	if err != nil {
		return nil, err
	}

	namespace, err := v1.NewNamespace(ctx, "namespace", &v1.NamespaceArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name: pulumi.String(namespaceName),
		},
	}, pulumi.Provider(k8sProvider), pulumi.RetainOnDelete(true))
	if err != nil {
		return nil, fmt.Errorf("failed to create namespace: %w", err)
	}

	return &k8sSetup{
		Provider:  k8sProvider,
		Namespace: namespace,
	}, nil
}
