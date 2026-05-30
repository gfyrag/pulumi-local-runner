package main

import (
	"fmt"

	"github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

// k8sSetup holds the common Kubernetes provider and namespace setup.
type k8sSetup struct {
	Provider      pulumi.ProviderResource
	NamespaceName string
}

// newK8sSetup creates a Kubernetes provider from config.
func newK8sSetup(ctx *pulumi.Context, cfg *config.Config) (*k8sSetup, error) {
	kubeContext := cfg.Require("k8s-context")

	namespaceName := cfg.Get("namespace")
	if namespaceName == "" {
		namespaceName = ctx.Stack()
	}

	k8sProvider, err := kubernetes.NewProvider(ctx, "k8s", &kubernetes.ProviderArgs{
		Context: pulumi.StringPtr(kubeContext),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create k8s provider: %w", err)
	}

	return &k8sSetup{
		Provider:      k8sProvider,
		NamespaceName: namespaceName,
	}, nil
}
