// Deploy a LedgerService custom resource from Pulumi config.
package main

import (
	"fmt"

	"github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes"
	"github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/apiextensions"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cfg := config.New(ctx, "")

		kubeContext := cfg.Require("k8s-context")
		namespace := cfg.Require("namespace")
		name := cfg.Require("name")

		k8sProvider, err := kubernetes.NewProvider(ctx, "k8s", &kubernetes.ProviderArgs{
			Context: pulumi.StringPtr(kubeContext),
		})
		if err != nil {
			return fmt.Errorf("failed to create k8s provider: %w", err)
		}

		spec, err := loadSpec(cfg)
		if err != nil {
			return fmt.Errorf("failed to load LedgerService spec: %w", err)
		}

		ledgerSvc, err := apiextensions.NewCustomResource(ctx, "ledger-service", &apiextensions.CustomResourceArgs{
			ApiVersion: pulumi.String("ledger.formance.com/v1alpha1"),
			Kind:       pulumi.String("LedgerService"),
			Metadata: &metav1.ObjectMetaArgs{
				Name:      pulumi.String(name),
				Namespace: pulumi.String(namespace),
				Annotations: pulumi.StringMap{
					"pulumi.com/patchForce": pulumi.String("true"),
				},
			},
			OtherFields: kubernetes.UntypedArgs{
				"spec": spec,
			},
		},
			pulumi.Provider(k8sProvider),
		)
		if err != nil {
			return fmt.Errorf("failed to create LedgerService: %w", err)
		}

		// Create LedgerBackup if backup config is provided.
		backupSpec, err := loadBackup(cfg, name)
		if err != nil {
			return fmt.Errorf("failed to load backup config: %w", err)
		}

		if backupSpec != nil {
			_, err = apiextensions.NewCustomResource(ctx, "ledger-backup", &apiextensions.CustomResourceArgs{
				ApiVersion: pulumi.String("ledger.formance.com/v1alpha1"),
				Kind:       pulumi.String("LedgerBackup"),
				Metadata: &metav1.ObjectMetaArgs{
					Name:      pulumi.String(name),
					Namespace: pulumi.String(namespace),
					Annotations: pulumi.StringMap{
						"pulumi.com/patchForce": pulumi.String("true"),
					},
				},
				OtherFields: kubernetes.UntypedArgs{
					"spec": backupSpec,
				},
			},
				pulumi.Provider(k8sProvider),
				pulumi.DependsOn([]pulumi.Resource{ledgerSvc}),
			)
			if err != nil {
				return fmt.Errorf("failed to create LedgerBackup: %w", err)
			}
		}

		ctx.Export("name", pulumi.String(name))
		ctx.Export("namespace", pulumi.String(namespace))

		return nil
	})
}
