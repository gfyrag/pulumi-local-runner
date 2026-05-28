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

		values, err := shared.GetConfigObject(cfg, "nats", ".")
		if err != nil {
			return fmt.Errorf("failed to read NATS values: %w", err)
		}

		if values == nil {
			values = make(map[string]any)
		}

		release, err := helm.NewRelease(ctx, "nats", &helm.ReleaseArgs{
			Name:           pulumi.String("nats"),
			Chart:          pulumi.String("nats"),
			RepositoryOpts: &helm.RepositoryOptsArgs{Repo: pulumi.String("https://nats-io.github.io/k8s/helm/charts/")},
			Namespace:       k8s.NamespaceName,
			CreateNamespace: pulumi.Bool(false),
			Values:          pulumi.ToMap(values),
			ForceUpdate:     pulumi.Bool(true),
		},
			pulumi.Provider(k8s.Provider),
		)
		if err != nil {
			return fmt.Errorf("failed to deploy NATS: %w", err)
		}

		ctx.Export("release", release.Name)

		return nil
	})
}
