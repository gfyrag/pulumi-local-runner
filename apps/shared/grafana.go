package shared

import (
	"fmt"
	"strings"

	"github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/apiextensions"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

const GrafanaInstanceSelector = "grafana"

// GrafanaDatasourceSpec describes a Grafana datasource to create.
type GrafanaDatasourceSpec struct {
	Name      string
	Type      string
	URL       string
	IsDefault bool
	JsonData  map[string]any
}

// NewGrafanaDatasource creates a GrafanaDatasource CRD in the given namespace.
func NewGrafanaDatasource(
	ctx *pulumi.Context,
	k8s *K8sSetup,
	spec GrafanaDatasourceSpec,
	opts ...pulumi.ResourceOption,
) (*apiextensions.CustomResource, error) {
	dsSpec := map[string]any{
		"name":      spec.Name,
		"type":      spec.Type,
		"access":    "proxy",
		"url":       spec.URL,
		"isDefault": spec.IsDefault,
		"editable":  true,
	}
	if spec.JsonData != nil {
		dsSpec["jsonData"] = spec.JsonData
	}

	k8sName := fmt.Sprintf("datasource-%s", strings.ToLower(spec.Name))

	baseOpts := []pulumi.ResourceOption{
		pulumi.DependsOn([]pulumi.Resource{k8s.Namespace}),
		pulumi.Provider(k8s.Provider),
	}

	return apiextensions.NewCustomResource(ctx, k8sName, &apiextensions.CustomResourceArgs{
		ApiVersion: pulumi.String("grafana.integreatly.org/v1beta1"),
		Kind:       pulumi.String("GrafanaDatasource"),
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String(k8sName),
			Namespace: k8s.Namespace.Metadata.Name(),
		},
		OtherFields: map[string]any{
			"spec": map[string]any{
				"instanceSelector": map[string]any{
					"matchLabels": map[string]any{
						"dashboards": GrafanaInstanceSelector,
					},
				},
				"datasource": dsSpec,
			},
		},
	}, append(baseOpts, opts...)...)
}
