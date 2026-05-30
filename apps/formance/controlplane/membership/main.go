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

		k8s, err := newK8sSetup(ctx, cfg)
		if err != nil {
			return err
		}
		k8sProvider := k8s.Provider

		// Clone source repository
		sourceRepo := cfg.Get("source-repo")
		if sourceRepo == "" {
			sourceRepo = "git@github.com:formancehq/membership-api.git"
		}
		sourceRef := cfg.Get("source-ref")

		sourceDir, err := shared.CloneSourceRepo(sourceRepo, sourceRef)
		if err != nil {
			return fmt.Errorf("failed to clone source repo: %w", err)
		}

		dc := shared.NewDockerConfig(ctx, cfg, sourceDir)
		domain := cfg.Require("domain")

		natsURL := cfg.Get("nats-url")
		if natsURL == "" {
			natsURL = "nats://nats:4222"
		}

		// Build membership image (context = cloned repo, Dockerfile = local)
		membershipImage, err := dc.BuildImage(ctx, "formancehq/membership", sourceDir, "./Dockerfile")
		if err != nil {
			return fmt.Errorf("failed to build membership image: %w", err)
		}

		// Build Helm values
		helmValues := pulumi.Map{
			"image": pulumi.Map{
				"repository": pulumi.Sprintf("%s/formancehq/membership", dc.PullRegistry),
				"tag":        pulumi.Sprintf("latest@%s", membershipImage.Digest),
			},
			"global": pulumi.Map{
				"debug":       pulumi.Bool(cfg.GetBool("debug")),
				"serviceHost": pulumi.String(domain),
				"nats": pulumi.Map{
					"enabled": pulumi.Bool(true),
					"url":     pulumi.String(natsURL),
				},
			},
			"feature": pulumi.Map{
				"managedStacks":  pulumi.Bool(false),
				"disableEvents":  pulumi.Bool(false),
				"migrationHooks": pulumi.Bool(true),
			},
			"config": pulumi.Map{
				"grpc": pulumi.Map{
					"tokens": shared.GetConfigStringArray(cfg, "grpc-tokens"),
				},
				"additionalEnv": pulumi.Array{
					pulumi.Map{
						"name":  pulumi.String("DISABLE_DEFAULT_MODULES"),
						"value": pulumi.String("true"),
					},
				},
			},
			"ingress": shared.IngressValues(cfg, "ingress"),
			"dex": pulumi.Map{
				"ingress": shared.IngressValues(cfg, "dex-ingress"),
			},
			"imagePullSecrets": shared.GetImagePullSecrets(cfg),
			"nodeSelector":     shared.GetConfigMap(cfg, "node-selector"),
			"tolerations":      shared.GetConfigArray(cfg, "tolerations"),
		}

		// Deploy membership via Helm chart from formancehq/helm repo
		membershipRelease, err := helm.NewRelease(ctx, "membership", &helm.ReleaseArgs{
			Name:        pulumi.String("membership"),
			Chart:       pulumi.String("oci://ghcr.io/formancehq/helm/membership"),
			Namespace:   pulumi.String(k8s.NamespaceName),
			Values:      helmValues,
			ForceUpdate: pulumi.Bool(true),
		},
			pulumi.DependsOn([]pulumi.Resource{membershipImage.Resource()}),
			pulumi.Provider(k8sProvider),
		)
		if err != nil {
			return fmt.Errorf("failed to deploy membership: %w", err)
		}

		// Exports
		ctx.Export("namespace", pulumi.String(k8s.NamespaceName))
		ctx.Export("membershipImage", pulumi.Sprintf("%s/formancehq/membership:latest@%s", dc.PullRegistry, membershipImage.Digest))
		ctx.Export("membershipRelease", membershipRelease.Name)

		return nil
	})
}
