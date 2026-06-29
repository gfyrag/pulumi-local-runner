// Operator deployment stack: builds operator images, applies CRDs, deploys operator.
package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gfyrag/pulumi-local-runner/apps/shared"
	v1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/core/v1"
	"github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/helm/v3"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	k8syaml "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/yaml"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cfg := config.New(ctx, "")

		k8s, err := newK8sSetup(ctx, cfg)
		if err != nil {
			return err
		}
		namespace := k8s.Namespace
		k8sProvider := k8s.Provider

		sourceRepo := cfg.Get("source-repo")
		if sourceRepo == "" {
			sourceRepo = "git@github.com:formancehq/ledger.git"
		}
		sourceRef := cfg.Get("source-ref")

		sourceDir, err := shared.CloneSourceRepo(sourceRepo, sourceRef)
		if err != nil {
			return fmt.Errorf("failed to clone source repo: %w", err)
		}
		operatorSrc := filepath.Join(sourceDir, "misc", "operator")

		dc := shared.NewDockerConfig(ctx, cfg, sourceDir)

		// Build operator image
		ledgerOperatorImage, err := dc.BuildImage(ctx, "formancehq/ledger-operator", operatorSrc, filepath.Join(operatorSrc, "Dockerfile"))
		if err != nil {
			return fmt.Errorf("failed to build ledger operator image: %w", err)
		}

		// Apply CRDs
		crdFiles, err := filepath.Glob(filepath.Join(operatorSrc, "helm", "crds", "templates", "*.yaml"))
		if err != nil {
			return fmt.Errorf("failed to glob CRD files: %w", err)
		}

		var ledgerCRDs []pulumi.Resource
		for _, crdFile := range crdFiles {
			name := strings.TrimSuffix(filepath.Base(crdFile), filepath.Ext(crdFile))
			crd, crdErr := k8syaml.NewConfigFile(ctx, name+"-crd", &k8syaml.ConfigFileArgs{
				File: crdFile,
			}, pulumi.Provider(k8sProvider))
			if crdErr != nil {
				return fmt.Errorf("failed to apply CRD %s: %w", name, crdErr)
			}
			ledgerCRDs = append(ledgerCRDs, crd)
		}

		// Ledger image configuration
		ledgerImageRegistry := cfg.Get("ledger-image-registry")
		if ledgerImageRegistry == "" {
			ledgerImageRegistry = dc.PullRegistry
		}
		ledgerImageName := cfg.Get("ledger-image-name")
		if ledgerImageName == "" {
			ledgerImageName = "formancehq/ledger"
		}
		ledgerImageTag := cfg.Get("ledger-image-tag")
		if ledgerImageTag == "" {
			ledgerImageTag = "latest"
		}

		// Operator runtime configuration
		imagePullPolicy := cfg.Get("image-pull-policy")
		if imagePullPolicy == "" {
			imagePullPolicy = "IfNotPresent"
		}
		replicaCount := 1
		if v, err := cfg.TryInt("replicaCount"); err == nil {
			replicaCount = v
		}
		leaderElection := true
		if v, err := cfg.TryBool("leaderElection"); err == nil {
			leaderElection = v
		}
		var watchNamespace pulumi.StringInput = namespace.Metadata.Name().Elem()
		if v := cfg.Get("watchNamespace"); v != "" {
			watchNamespace = pulumi.String(v)
		}

		// Deploy ledger operator
		operatorChartPath := filepath.Join(operatorSrc, "helm", "operator")

		helmValues := pulumi.Map{
			"image": pulumi.Map{
				"repository": pulumi.Sprintf("%s/formancehq/ledger-operator", dc.PullRegistry),
				"tag":        pulumi.Sprintf("latest@%s", ledgerOperatorImage.Digest),
				"pullPolicy": pulumi.String(imagePullPolicy),
			},
			"ledgerImage": pulumi.Map{
				"registry": pulumi.String(ledgerImageRegistry),
				"name":     pulumi.String(ledgerImageName),
				"tag":      pulumi.String(ledgerImageTag),
			},
			"imagePullSecrets": shared.GetImagePullSecrets(cfg),
			"replicaCount":     pulumi.Int(replicaCount),
			"leaderElection":   pulumi.Bool(leaderElection),
			"watchNamespace":   watchNamespace,
			"nodeSelector":     shared.GetConfigMap(cfg, "node-selector"),
			"tolerations":      shared.GetConfigArray(cfg, "tolerations"),
			"ledger-operator-crds": pulumi.Map{
				"create": pulumi.Bool(false),
			},
		}
		if v := shared.GetConfigMap(cfg, "resources"); len(v) > 0 {
			helmValues["resources"] = v
		}
		if v := shared.GetConfigMap(cfg, "serviceAccount"); len(v) > 0 {
			helmValues["serviceAccount"] = v
		}
		if v := shared.GetConfigMap(cfg, "pvcProtection"); len(v) > 0 {
			helmValues["pvcProtection"] = v
		}

		ledgerOperator, err := helm.NewRelease(ctx, "ledger-operator", &helm.ReleaseArgs{
			Name:             pulumi.String("ledger-operator"),
			Chart:            pulumi.String(operatorChartPath),
			Namespace:        namespace.Metadata.Name(),
			Values:           helmValues,
			ForceUpdate:      pulumi.Bool(true),
			DependencyUpdate: pulumi.Bool(true),
		},
			pulumi.DependsOn(append([]pulumi.Resource{namespace, ledgerOperatorImage.Resource()}, ledgerCRDs...)),
			pulumi.Provider(k8sProvider),
		)
		if err != nil {
			return fmt.Errorf("failed to deploy ledger operator: %w", err)
		}

		// ServiceAccount for cold storage IRSA (optional)
		coldStorageRoleARN := cfg.Get("coldStorage-iam-role-arn")
		if coldStorageRoleARN != "" {
			_, err = v1.NewServiceAccount(ctx, "aws-access", &v1.ServiceAccountArgs{
				Metadata: &metav1.ObjectMetaArgs{
					Name:      pulumi.String("aws-access"),
					Namespace: namespace.Metadata.Name(),
					Annotations: pulumi.StringMap{
						"eks.amazonaws.com/role-arn": pulumi.String(coldStorageRoleARN),
					},
				},
			}, pulumi.Provider(k8sProvider))
			if err != nil {
				return fmt.Errorf("failed to create IRSA ServiceAccount: %w", err)
			}
		}

		// Exports
		ctx.Export("namespace", namespace.Metadata.Name())
		ctx.Export("ledgerOperatorImage", pulumi.Sprintf("%s/formancehq/ledger-operator:latest@%s", dc.PullRegistry, ledgerOperatorImage.Digest))
		ctx.Export("ledgerOperatorRelease", ledgerOperator.Name)
		ctx.Export("ledgerImage", pulumi.String(ledgerImageRegistry+"/"+ledgerImageName+":"+ledgerImageTag))
		if coldStorageRoleARN != "" {
			ctx.Export("coldStorageRoleArn", pulumi.String(coldStorageRoleARN))
		}
		coldStorageBucket := cfg.Get("coldStorage-s3-bucket")
		if coldStorageBucket != "" {
			ctx.Export("coldStorageBucket", pulumi.String(coldStorageBucket))
		}

		return nil
	})
}
