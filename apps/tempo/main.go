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

		values, err := shared.GetConfigObject(cfg, "tempo", ".")
		if err != nil {
			return fmt.Errorf("failed to read Tempo values: %w", err)
		}

		release, err := helm.NewRelease(ctx, "tempo", &helm.ReleaseArgs{
			Name:           pulumi.String("tempo"),
			Chart:          pulumi.String("tempo"),
			RepositoryOpts: &helm.RepositoryOptsArgs{Repo: pulumi.String("https://grafana.github.io/helm-charts")},
			Namespace:      k8s.Namespace.Metadata.Name(),
			Values:         pulumi.ToMap(values),
			ForceUpdate:    pulumi.Bool(true),
		},
			pulumi.DependsOn([]pulumi.Resource{k8s.Namespace}),
			pulumi.Provider(k8s.Provider),
		)
		if err != nil {
			return fmt.Errorf("failed to deploy Tempo: %w", err)
		}

		_, err = shared.NewGrafanaDatasource(ctx, k8s, shared.GrafanaDatasourceSpec{
			Name: "Tempo",
			Type: "tempo",
			URL:  "http://tempo:3200",
		}, pulumi.DependsOn([]pulumi.Resource{release}))
		if err != nil {
			return fmt.Errorf("failed to create Tempo datasource: %w", err)
		}

		ctx.Export("release", release.Name)

		return nil
	})
}
