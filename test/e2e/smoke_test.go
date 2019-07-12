package e2etest

import (
	"fmt"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	// Uncomment the following line to load the gcp plugin (only required to authenticate against GKE clusters).
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

// TestSmoke makes sure we can run tests.
func TestSmoke(t *testing.T) {
	client := Setup(t, true)
	defer TearDown(client)

	ns := "default"

	_, filename, _, _ := runtime.Caller(0)

	installer := NewInstaller(ns, client.Dynamic, fmt.Sprintf("%s/config/smoketest/", filepath.Dir(filename)))

	// Delete deferred.
	defer func() {
		if err := installer.Do("delete"); err != nil {
			t.Errorf("failed to create, %s", err)
		}
	}()

	// Create the resources for the test.
	if err := installer.Do("create"); err != nil {
		t.Errorf("failed to create, %s", err)
	}

	// TODO: verify stuff...
	time.Sleep(2 * time.Second)
}
