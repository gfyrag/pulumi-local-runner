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

		values, err := shared.GetConfigObject(cfg, "pyroscope", ".")
		if err != nil {
			return fmt.Errorf("failed to read Pyroscope values: %w", err)
		}

		release, err := helm.NewRelease(ctx, "pyroscope", &helm.ReleaseArgs{
			Name:           pulumi.String("pyroscope"),
			Chart:          pulumi.String("pyroscope"),
			RepositoryOpts: &helm.RepositoryOptsArgs{Repo: pulumi.String("https://grafana.github.io/helm-charts")},
			Namespace:       k8s.NamespaceName,
			CreateNamespace: pulumi.Bool(false),
			Values:          pulumi.ToMap(values),
			ForceUpdate:     pulumi.Bool(true),
		},
			pulumi.Provider(k8s.Provider),
		)
		if err != nil {
			return fmt.Errorf("failed to deploy Pyroscope: %w", err)
		}

		_, err = shared.NewGrafanaDatasource(ctx, k8s, shared.GrafanaDatasourceSpec{
			Name: "Pyroscope",
			Type: "grafana-pyroscope-datasource",
			URL:  "http://pyroscope:4040",
		}, pulumi.DependsOn([]pulumi.Resource{release}))
		if err != nil {
			return fmt.Errorf("failed to create Pyroscope datasource: %w", err)
		}

		ctx.Export("release", release.Name)

		return nil
	})
}
