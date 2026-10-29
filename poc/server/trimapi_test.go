package server

import (
	"context"
	"testing"
)

func TestTrimCallRequiresToken(t *testing.T) {
	t.Setenv("TRIM_API_TOKEN", "")
	if _, err := trimCall(context.Background(), "trim.file.getSharedAccessibleFolders", map[string]any{}); err == nil {
		t.Fatal("expected an error when the fnOS gateway token is absent")
	}
}

func TestTrimAppNameDefault(t *testing.T) {
	t.Setenv("TRIM_APPNAME", "")
	if got := trimAppName(); got != "qas" {
		t.Fatalf("trimAppName() = %q, want qas", got)
	}
}
