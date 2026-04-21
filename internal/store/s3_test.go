package store

import (
	"testing"
)

func TestS3StoreKey(t *testing.T) {
	s := &S3Store{prefix: "team/"}
	if got := s.appFileKey("myapp"); got != "team/apps/myapp/app.yaml" {
		t.Errorf("appFileKey() = %q, want %q", got, "team/apps/myapp/app.yaml")
	}
	if got := s.stackFileKey("myapp", "dev"); got != "team/apps/myapp/dev.yaml" {
		t.Errorf("stackFileKey() = %q, want %q", got, "team/apps/myapp/dev.yaml")
	}
	if got := s.stackConfigKey("myapp", "dev"); got != "team/apps/myapp/dev.yaml" {
		t.Errorf("stackConfigKey() = %q, want %q", got, "team/apps/myapp/dev.yaml")
	}
}

func TestS3StoreKeyNoPrefix(t *testing.T) {
	s := &S3Store{prefix: ""}
	if got := s.appFileKey("myapp"); got != "apps/myapp/app.yaml" {
		t.Errorf("appFileKey() = %q, want %q", got, "apps/myapp/app.yaml")
	}
	if got := s.stackConfigKey("app", "prod"); got != "apps/app/prod.yaml" {
		t.Errorf("stackConfigKey() = %q, want %q", got, "apps/app/prod.yaml")
	}
}

func TestS3StoreKeyPrefixNoTrailingSlash(t *testing.T) {
	s := &S3Store{prefix: "team"}
	if got := s.appFileKey("myapp"); got != "team/apps/myapp/app.yaml" {
		t.Errorf("appFileKey() = %q, want %q", got, "team/apps/myapp/app.yaml")
	}
}
