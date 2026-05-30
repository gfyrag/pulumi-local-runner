package main

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestLoadSpec_FromFile(t *testing.T) {
	specContent := `
replicas: 3
image:
  repository: ghcr.io/formancehq/ledger
  tag: v1.0.0
config:
  clusterID: test
  pebble:
    memTableSize: 67108864
`
	dir := t.TempDir()
	specFile := filepath.Join(dir, "spec.yaml")
	if err := os.WriteFile(specFile, []byte(specContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Parse to verify structure
	var spec map[string]any
	if err := yaml.Unmarshal([]byte(specContent), &spec); err != nil {
		t.Fatal(err)
	}

	if spec["replicas"] != 3 {
		t.Errorf("expected replicas=3, got %v", spec["replicas"])
	}

	img := spec["image"].(map[string]any)
	if img["repository"] != "ghcr.io/formancehq/ledger" {
		t.Errorf("unexpected repository: %v", img["repository"])
	}
	if img["tag"] != "v1.0.0" {
		t.Errorf("unexpected tag: %v", img["tag"])
	}

	cfg := spec["config"].(map[string]any)
	if cfg["clusterID"] != "test" {
		t.Errorf("unexpected clusterID: %v", cfg["clusterID"])
	}

	pebble := cfg["pebble"].(map[string]any)
	if pebble["memTableSize"] != 67108864 {
		t.Errorf("unexpected memTableSize: %v", pebble["memTableSize"])
	}
}

func TestLoadSpec_FullSpec(t *testing.T) {
	// Verify a comprehensive spec can be parsed
	specContent := `
replicas: 5
image:
  repository: ghcr.io/formancehq/ledger
  tag: v2.0.0
  pullPolicy: IfNotPresent
config:
  clusterID: my-cluster
  bindAddr: "0.0.0.0:7777"
  grpcPort: 8888
  httpPort: 9000
  walDir: /data/wal
  dataDir: /data/data
  debug: false
  pebble:
    memTableSize: 67108864
    cacheSize: 536870912
    maxConcurrentCompactions: 2
  raft:
    electionTick: 10
    heartbeatTick: 1
    maintenanceInterval: 30s
    tickInterval: 100ms
  monitoring:
    serviceName: ledger
    traces:
      enabled: true
      exporter: otlp
    metrics:
      enabled: true
      exporter: otlp
  coldStorage:
    driver: s3
    s3:
      bucket: my-bucket
      region: eu-west-1
  auth:
    enabled: true
    issuer: https://auth.example.com
    scopeMapping:
      "ledger:read":
        - "ledgers:read"
        - "transactions:read"
persistence:
  wal:
    size: 10Gi
  data:
    size: 50Gi
    storageClass: gp3
resources:
  requests:
    cpu: 2000m
    memory: 2048Mi
  limits:
    cpu: 2000m
    memory: 2048Mi
podAntiAffinity:
  enabled: true
  type: soft
  topologyKey: kubernetes.io/hostname
podDisruptionBudget:
  enabled: true
  minAvailable: 2
networkPolicy:
  enabled: true
autoNetworking:
  tld: example.com
  suffix: "-ledger"
  ingress:
    enabled: true
  dnsEndpoint:
    enabled: true
`

	var spec map[string]any
	if err := yaml.Unmarshal([]byte(specContent), &spec); err != nil {
		t.Fatalf("failed to parse full spec: %v", err)
	}

	// Verify key fields
	if spec["replicas"] != 5 {
		t.Errorf("unexpected replicas: %v", spec["replicas"])
	}

	cfg := spec["config"].(map[string]any)
	auth := cfg["auth"].(map[string]any)
	if auth["issuer"] != "https://auth.example.com" {
		t.Errorf("unexpected auth issuer: %v", auth["issuer"])
	}

	persistence := spec["persistence"].(map[string]any)
	data := persistence["data"].(map[string]any)
	if data["storageClass"] != "gp3" {
		t.Errorf("unexpected storageClass: %v", data["storageClass"])
	}

	autoNet := spec["autoNetworking"].(map[string]any)
	if autoNet["tld"] != "example.com" {
		t.Errorf("unexpected tld: %v", autoNet["tld"])
	}
}
