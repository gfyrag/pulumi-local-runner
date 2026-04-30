package main

import (
	"fmt"

	"github.com/formancehq/ledger-v3-poc/deployments/devenv/shared"
	"github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/apiextensions"
	"github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/helm/v3"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

const grafanaInstanceSelector = "grafana"

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cfg := config.New(ctx, "")

		k8s, err := shared.NewK8sSetup(ctx, cfg)
		if err != nil {
			return err
		}

		// Grafana Operator
		grafanaOperator, err := helm.NewRelease(ctx, "grafana-operator", &helm.ReleaseArgs{
			Name:           pulumi.String("grafana-operator"),
			Chart:          pulumi.String("grafana-operator"),
			RepositoryOpts: &helm.RepositoryOptsArgs{Repo: pulumi.String("https://grafana.github.io/helm-charts")},
			Namespace:      k8s.NamespaceName,
			ForceUpdate:    pulumi.Bool(true),
		},
			pulumi.Provider(k8s.Provider),
		)
		if err != nil {
			return fmt.Errorf("failed to deploy Grafana Operator: %w", err)
		}

		// Grafana instance (CRD)
		grafanaSpec, err := shared.GetConfigObject(cfg, "grafana", ".")
		if err != nil {
			return fmt.Errorf("failed to read Grafana values: %w", err)
		}

		grafana, err := apiextensions.NewCustomResource(ctx, "grafana", &apiextensions.CustomResourceArgs{
			ApiVersion: pulumi.String("grafana.integreatly.org/v1beta1"),
			Kind:       pulumi.String("Grafana"),
			Metadata: &metav1.ObjectMetaArgs{
				Name:      pulumi.String("grafana"),
				Namespace: k8s.NamespaceName,
				Labels: pulumi.StringMap{
					"dashboards": pulumi.String(grafanaInstanceSelector),
				},
			},
			OtherFields: map[string]any{
				"spec": grafanaSpec,
			},
		},
			pulumi.DependsOn([]pulumi.Resource{grafanaOperator}),
			pulumi.Provider(k8s.Provider),
		)
		if err != nil {
			return fmt.Errorf("failed to create Grafana instance: %w", err)
		}

		// Grafana DNSEndpoint (optional, for ExternalDNS)
		var grafanaDNSEndpointConfig map[string]any
		if err := cfg.GetObject("grafana-dnsEndpoint", &grafanaDNSEndpointConfig); err == nil && grafanaDNSEndpointConfig != nil {
			endpoints, _ := grafanaDNSEndpointConfig["endpoints"].([]any)

			var dnsAnnotations map[string]string
			if raw, ok := grafanaDNSEndpointConfig["annotations"].(map[string]any); ok {
				dnsAnnotations = make(map[string]string, len(raw))
				for k, v := range raw {
					dnsAnnotations[k] = fmt.Sprintf("%v", v)
				}
			}

			_, err = apiextensions.NewCustomResource(ctx, "grafana-dnsendpoint", &apiextensions.CustomResourceArgs{
				ApiVersion: pulumi.String("externaldns.k8s.io/v1alpha1"),
				Kind:       pulumi.String("DNSEndpoint"),
				Metadata: &metav1.ObjectMetaArgs{
					Name:        pulumi.String("grafana"),
					Namespace:   k8s.NamespaceName,
					Annotations: pulumi.ToStringMap(dnsAnnotations),
				},
				OtherFields: map[string]any{
					"spec": map[string]any{
						"endpoints": endpoints,
					},
				},
			},
				pulumi.DependsOn([]pulumi.Resource{grafana}),
				pulumi.Provider(k8s.Provider),
			)
			if err != nil {
				return fmt.Errorf("failed to create Grafana DNSEndpoint: %w", err)
			}
		}

		// Exports
		ctx.Export("grafanaOperatorRelease", grafanaOperator.Name)

		return nil
	})
}
