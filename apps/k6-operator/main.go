package main

import (
	"fmt"

	"github.com/gfyrag/pulumi-local-runner/apps/shared"
	"github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/helm/v3"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cfg := config.New(ctx, "")

		k8s, err := shared.NewK8sSetup(ctx, cfg)
		if err != nil {
			return err
		}

		values, err := shared.GetConfigObject(cfg, "k6-operator", ".")
		if err != nil {
			return fmt.Errorf("failed to read k6 Operator values: %w", err)
		}

		// The k6-operator chart hardcodes its workloads (CRDs, RBAC, manager
		// Deployment) into a `<release-name>-system` namespace that it creates
		// itself when `namespace.create=true` (chart default). It does not
		// expose a way to redirect those resources to another namespace, so we
		// let the chart manage that namespace and only use `Namespace` below
		// for the release storage object.

		release, err := helm.NewRelease(ctx, "k6-operator", &helm.ReleaseArgs{
			Name:           pulumi.String("k6-operator"),
			Chart:          pulumi.String("k6-operator"),
			RepositoryOpts: &helm.RepositoryOptsArgs{Repo: pulumi.String("https://grafana.github.io/helm-charts")},
			Namespace:       k8s.NamespaceName,
			CreateNamespace: pulumi.Bool(false),
			Values:          pulumi.ToMap(values),
			ForceUpdate:     pulumi.Bool(true),
		},
			pulumi.Provider(k8s.Provider),
		)
		if err != nil {
			return fmt.Errorf("failed to deploy k6 Operator: %w", err)
		}

		ctx.Export("release", release.Name)

		return nil
	})
}
