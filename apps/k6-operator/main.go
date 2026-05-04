package main

import (
	"fmt"

	"github.com/formancehq/ledger-v3-poc/deployments/devenv/shared"
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

		// Disable namespace creation — the namespace is managed externally
		if values == nil {
			values = make(map[string]any)
		}
		ns, _ := values["namespace"].(map[string]any)
		if ns == nil {
			ns = make(map[string]any)
		}
		ns["create"] = false
		values["namespace"] = ns

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
