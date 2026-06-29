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

		values, err := shared.GetConfigObject(cfg, "loki", ".")
		if err != nil {
			return fmt.Errorf("failed to read Loki values: %w", err)
		}

		release, err := helm.NewRelease(ctx, "loki", &helm.ReleaseArgs{
			Name:           pulumi.String("loki"),
			Chart:          pulumi.String("loki"),
			RepositoryOpts: &helm.RepositoryOptsArgs{Repo: pulumi.String("https://grafana.github.io/helm-charts")},
			Namespace:       k8s.NamespaceName,
			CreateNamespace: pulumi.Bool(false),
			Values:          pulumi.ToMap(values),
			ForceUpdate:     pulumi.Bool(true),
		},
			pulumi.Provider(k8s.Provider),
		)
		if err != nil {
			return fmt.Errorf("failed to deploy Loki: %w", err)
		}

		_, err = shared.NewGrafanaDatasource(ctx, k8s, shared.GrafanaDatasourceSpec{
			Name: "Loki",
			Type: "loki",
			URL:  "http://loki:3100",
		}, pulumi.DependsOn([]pulumi.Resource{release}))
		if err != nil {
			return fmt.Errorf("failed to create Loki datasource: %w", err)
		}

		ctx.Export("release", release.Name)

		return nil
	})
}
