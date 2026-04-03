package store

import (
	"testing"
)

func TestS3StoreKey(t *testing.T) {
	s := &S3Store{prefix: "team/"}
	if got := s.configKey(); got != "team/config.yaml" {
		t.Errorf("configKey() = %q, want %q", got, "team/config.yaml")
	}
	if got := s.stackConfigKey("myapp", "dev"); got != "team/stacks/myapp/Pulumi.dev.yaml" {
		t.Errorf("stackConfigKey() = %q, want %q", got, "team/stacks/myapp/Pulumi.dev.yaml")
	}
}

func TestS3StoreKeyNoPrefix(t *testing.T) {
	s := &S3Store{prefix: ""}
	if got := s.configKey(); got != "config.yaml" {
		t.Errorf("configKey() = %q, want %q", got, "config.yaml")
	}
	if got := s.stackConfigKey("app", "prod"); got != "stacks/app/Pulumi.prod.yaml" {
		t.Errorf("stackConfigKey() = %q, want %q", got, "stacks/app/Pulumi.prod.yaml")
	}
}

func TestS3StoreKeyPrefixNoTrailingSlash(t *testing.T) {
	s := &S3Store{prefix: "team"}
	if got := s.configKey(); got != "team/config.yaml" {
		t.Errorf("configKey() = %q, want %q", got, "team/config.yaml")
	}
}
