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

		values, err := shared.GetConfigObject(cfg, "victoriametrics", ".")
		if err != nil {
			return fmt.Errorf("failed to read VictoriaMetrics values: %w", err)
		}

		release, err := helm.NewRelease(ctx, "victoriametrics", &helm.ReleaseArgs{
			Name:           pulumi.String("vm"),
			Chart:          pulumi.String("victoria-metrics-single"),
			RepositoryOpts: &helm.RepositoryOptsArgs{Repo: pulumi.String("https://victoriametrics.github.io/helm-charts/")},
			Namespace:      k8s.Namespace.Metadata.Name(),
			Values:         pulumi.ToMap(values),
			ForceUpdate:    pulumi.Bool(true),
		},
			pulumi.DependsOn([]pulumi.Resource{k8s.Namespace}),
			pulumi.Provider(k8s.Provider),
		)
		if err != nil {
			return fmt.Errorf("failed to deploy VictoriaMetrics: %w", err)
		}

		_, err = shared.NewGrafanaDatasource(ctx, k8s, shared.GrafanaDatasourceSpec{
			Name:      "VictoriaMetrics",
			Type:      "prometheus",
			URL:       "http://vm-victoria-metrics-single-server:8428",
			IsDefault: true,
			JsonData: map[string]any{
				"httpMethod":        "POST",
				"prometheusType":    "Prometheus",
				"prometheusVersion": "2.37.0",
			},
		}, pulumi.DependsOn([]pulumi.Resource{release}))
		if err != nil {
			return fmt.Errorf("failed to create VictoriaMetrics datasource: %w", err)
		}

		ctx.Export("release", release.Name)

		return nil
	})
}
