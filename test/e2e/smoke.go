/*
Copyright 2019 Google LLC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package e2etest

import (
	"fmt"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"

	// Uncomment the following line to load the gcp plugin (only required to authenticate against GKE clusters).
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

// Smoke makes sure we can run tests.
func Smoke(t *testing.T) {
	client := Setup(t, true)
	defer TearDown(client)

	_, filename, _, _ := runtime.Caller(0)

	yamls := fmt.Sprintf("%s/config/smoketest/", filepath.Dir(filename))
	installer := NewInstaller(client.Dynamic, map[string]string{
		"namespace": client.Namespace,
	}, yamls)

	// Delete deferred.
	defer func() {
		// Just chill for tick.
		time.Sleep(20 * time.Second)
		if err := installer.Do("delete"); err != nil {
			t.Errorf("failed to create, %s", err)
		}
	}()

	// Create the resources for the test.
	if err := installer.Do("create"); err != nil {
		t.Errorf("failed to create, %s", err)
	}

	if err := client.WaitForResourceReady(client.Namespace, "e2e-smoke-test", schema.GroupVersionResource{
		Group:    "pubsub.cloud.run",
		Version:  "v1alpha1",
		Resource: "pullsubscriptions",
	}); err != nil {
		t.Error(err)
	}
}
